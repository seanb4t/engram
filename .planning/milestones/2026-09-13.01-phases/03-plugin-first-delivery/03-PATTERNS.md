# Phase 3: Plugin-First Delivery - Pattern Map

**Mapped:** 2026-09-14
**Files analyzed:** 12 (new + modified)
**Analogs found:** 12 / 12

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/setup/plugin.go` (NEW) | service (per-runtime decision+exec lane) | request-response (shell-out + read-then-decide-then-act) | `internal/setup/apply.go` (`execute`) | role-match (structurally different sequencing, same execution primitives) |
| `internal/setup/plugin_test.go` (NEW) | test | request-response | `internal/setup/apply_test.go` / `cmd/engram/setup_test.go`'s `fakeSetupEnvWithRun` idiom | role-match |
| `internal/setup/pluginversion.go` (NEW, or folded into `plugin.go`) | utility (stdlib-only comparator) | transform | `cmd/engram/buildversion.go` (`patchCorePattern`/`nextPatch`) | exact (same technique, adjacent problem) |
| `internal/setup/plan.go` (MODIFIED) | model (data-only Plan/Result shapes) | CRUD (struct field addition) | itself — `SkillFormat`/`SkillTarget`/`Outcome` enum idiom | exact (extend existing enum-with-doc-comment convention) |
| `internal/setup/claudecode.go` (MODIFIED) | controller (per-runtime Plan authoring) | request-response | itself — existing `Plan()` arms + Phase 2 header rendering | exact |
| `internal/setup/codex.go` (MODIFIED) | controller (per-runtime Plan authoring) | request-response | itself — existing `Plan()` arms + Codex decline pattern | exact |
| `internal/setup/apply.go` (MODIFIED) | service (shared executor, new plugin lane) | request-response | itself — `execute()`'s probe→actions→probe flow, `runSeam` | exact |
| `internal/skills/environment.go` (MODIFIED — new `Lstat` seam) | utility (injectable FS seam) | file-I/O | itself — existing `ReadFile`/`WriteFile`/`MkdirAll` fields | exact |
| `internal/skills/install.go` (MODIFIED — D-08 presence report) | service (skills install/report) | file-I/O | itself — `installFiles`/`installAgentsMDIndex` | exact |
| `cmd/engram/setup.go` (MODIFIED) | controller (CLI composition) | request-response | itself — `setupApplySkillsFacet`, `setupRuntimeRow`, exit taxonomy | exact |
| `cmd/engram/setup_test.go` (MODIFIED) | test | request-response | itself — `fakeSetupEnv`/`fakeSetupEnvWithRun`/`fakeSkillsEnv`/`withFakeSetupEnv` | exact |
| `cmd/engram/operator_view_setup_test.go` (MODIFIED — fixtures) | test | request-response | itself — `setupViewFixtures()` | exact |
| `skill/engram/.codex-plugin/plugin.json` (NEW) | config (manifest) | file-I/O | `skill/engram/.claude-plugin/plugin.json` | exact |
| `release-please-config.json` (MODIFIED) | config | batch (release-please sync) | itself — existing `extra-files` entry for `.claude-plugin/plugin.json` | exact |
| `internal/setupgen/setupgen.go` (MODIFIED, or `_test.go`) | service (codegen + drift gate) | transform | itself — `Cases()`/`Render()`/`adds` filter | exact |
| `skill/engram/commands/engram-setup.md` (regenerated) | config (generated doc) | transform | N/A — regenerate via `task surfaces:gen`, never hand-edit | n/a |

## Pattern Assignments

### `internal/setup/plugin.go` (NEW) — service, request-response

**Analogs:** `internal/setup/apply.go` (execution primitives: `runSeam`, `execTimeout`, `describeFailure`/`describeSeamError`), `internal/setup/runtime.go` (`optInOnlyRuntime` — the optional-interface capability-gate idiom).

**SPDX header** (copy verbatim onto every new Go file this phase adds):
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
```

**Package/import pattern** — `internal/setup` is a machine-gated stdlib-only leaf (see Shared Patterns below): `plugin.go`'s import block may use ONLY `context`, `encoding/json`, `errors`, `fmt`, `regexp`, `strconv`, `strings`, `time` — never anything with a dot in its first path segment (`internal/setup/leafpurity_test.go:118-121`, quoted below).

**Optional-interface capability gate** (`internal/setup/runtime.go:117-129`, read this session):
```go
// optInOnlyRuntime is an OPTIONAL interface a Runtime may implement to
// declare that it must never be included in Select(nil)'s default (no
// `--runtime`) selection...
type optInOnlyRuntime interface {
	OptInOnly() bool
}
```
Follow this EXACT shape for the plugin-capability interface: a small optional interface (e.g. `pluginCapableRuntime` with a method like `PluginProbe() []string` or similar) that only `claudeCodeRuntime`/`codexRuntime` implement, type-asserted once at the plugin lane's entry point in `plugin.go` — never a `rt.Name() == "claude-code"` branch anywhere (the package's own "no special-casing by name outside a runtime's own file" invariant, restated in `runtime.go`'s doc comment and `claudecode.go`/`codex.go`'s own headers).

**Bounded subprocess exec** (`internal/setup/apply.go:136-144`, read this session):
```go
func runSeam(ctx context.Context, env Environment, path string, args []string) (RunResult, error) {
	ctx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()
	rr, err := env.Run(ctx, path, args)
	if err != nil && errors.Is(err, context.DeadlineExceeded) {
		err = fmt.Errorf("timed out after %s: %w", execTimeout, err)
	}
	return rr, err
}
```
Reuse `runSeam` (and `execTimeout` = 20s, already package-level in `apply.go`) for the plugin lane's own probe/action execs — do not author a second timeout constant or a second bounded-exec helper (D-10's "bounded by the existing 20s execTimeout").

**Failure/legibility rendering** (`internal/setup/apply.go:155-170`):
```go
func describeFailure(name, cmdDisplay string, exitCode int, stderr string) string {
	reason := fmt.Sprintf("%s: %s exited %d", name, cmdDisplay, exitCode)
	if stderr != "" {
		reason += ": " + displayCapture(stderr)
	}
	return reason
}
func describeSeamError(name, cmdDisplay string, err error) string {
	return fmt.Sprintf("%s: %s: %v", name, cmdDisplay, err)
}
```
Reuse these two functions verbatim for any plugin-lane failure text (D-12's `unavailable: <reason>` string should compose the SAME way: reason text, never a diagnosed cause — "report, never diagnose", D-11's discipline stated throughout `apply.go`).

**JSON probe parsing** — no existing `internal/setup` file parses JSON yet (the package doc's `no-non-stdlib-import` constraint permits `encoding/json` since it is stdlib); model the parse on `internal/setup/generic.go`'s own `encoding/json` use (cited in RESEARCH.md's Standard Stack table) for the shape of a minimal, tolerant unmarshal into an anonymous/local struct — do not assume every documented JSON field exists (`claude plugin list --json`'s `mcpServers` is present on some entries, absent on others per RESEARCH.md's Code Examples).

---

### `internal/setup/pluginversion.go` (NEW, or folded into `plugin.go`) — utility, transform

**Analog:** `cmd/engram/buildversion.go:25-73` (verbatim precedent for the exact technique).

**Anchored-regex + strconv comparator pattern** (`cmd/engram/buildversion.go:25-34`, copy the TECHNIQUE, not the code — this file lives in `cmd/engram`, which is NOT the stdlib-only leaf, so it cannot be imported from `internal/setup`; re-author the same shape locally):
```go
// patchCorePattern anchors nextPatch's input to a plain, unsigned SemVer
// core: exactly three dot-separated components...
var patchCorePattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
```
```go
func nextPatch(v string) (string, bool) {
	m := patchCorePattern.FindStringSubmatch(v)
	if m == nil {
		return "", false
	}
	major, err := strconv.ParseUint(m[1], 10, 32)
	...
}
```
D-01/D-03's comparator needs a slightly different shape (COMPARE two version strings, not increment one), but the anchored-pattern + `strconv.ParseUint(..., 10, 32)` idiom is the exact reusable technique: match the same three-group anchored pattern against BOTH the installed-plugin version and `resolvedVersion()`'s output; a version that does NOT match (dev, `0.16.1-dev+sha`) is non-comparable (D-03: never triggers an update by comparison).

**Non-importable design reference** (do NOT import, read this session for shape only): `golang.org/x/mod/semver`'s exported API (`Compare`, `IsValid`, `Prerelease`) is a useful reference for what the local comparator's signature should expose, per RESEARCH.md's "Don't Hand-Roll" table — but see the Shared Pattern "stdlib-only leaf" below; this file MUST NOT import it.

**Comparand:** `cmd/engram/buildversion.go:141-166`'s `resolvedVersion()` is the SAME function `engram version` calls — it is the correct binary-side operand for D-01's comparison, never the raw `version` ldflags var (which is literally `"dev"` for a non-release build). Since `resolvedVersion()` lives in `cmd/engram` (not `internal/setup`), the comparison call site may need to thread the resolved version string INTO `internal/setup` as a parameter (e.g. on `Options` or a plugin-lane function argument) rather than calling `resolvedVersion()` from within the leaf package.

---

### `internal/setup/plan.go` (MODIFIED) — model, CRUD

**Analog:** itself — `SkillFormat`/`SkillTarget`/`Outcome` enum-with-doc-comment idiom (`internal/setup/plan.go:24-79`, read this session).

**Enum-with-explicit-values pattern** (`internal/setup/plan.go:57-79`):
```go
type SkillFormat string

const (
	SkillFormatNone SkillFormat = "none"
	SkillFormatNative SkillFormat = "native"
	SkillFormatAgentsMD SkillFormat = "agents-md"
)
```
Every value is a REAL, explicit value, never modeled as an absence or the zero value — the plugin-state vocabulary (absent/outdated/current/unavailable) must follow this SAME discipline: a new string-backed type (e.g. `PluginState`) with named constants and a doc comment on each, exactly like `SkillFormat`'s own comment block states the rule ("a zero-valued Outcome... is a programming error, never meaningful data").

**Growable-actions precedent** (`internal/setup/plan.go:129-134`, `Action`/`Plan.Actions` doc comments): `Plan.Actions` is documented as growable by a later phase without a type change — if the plan resolution favors folding plugin actions into `Plan.Actions` (Open Question 1 in RESEARCH.md), `Action{Args, Tolerant, Description}` (`plan.go:93-116`) needs no new fields; reuse it as-is.

**Struct-field addition, non-rendered field precedent** (`internal/setup/plan.go:230-252`, `Result.Skills SkillTarget` with `json:"-"`): if a plugin facet struct is threaded through `Result` the same way `Skills` is (so `cmd/engram` composes its own row fields from the SAME `Plan()`/`execute()` call rather than re-planning), give it the identical `json:"-"` non-rendered treatment — `cmd/engram` maps it onto flat scalar row fields (Shared Pattern below), never marshaling the struct itself.

---

### `internal/setup/claudecode.go` / `codex.go` (MODIFIED) — controller, request-response

**Analogs:** themselves — existing `Plan()` arms, `Detect()`, and the `codexRuntime.Plan()` up-front decline pattern.

**Per-runtime AUTHORED-HERE argv pattern** (`internal/setup/claudecode.go:84-90`, the tolerant remove action):
```go
var claudeCodeRemoveAction = Action{
	Args:     []string{"claude", "mcp", "remove", "engram", "--scope", "user"},
	Tolerant: true,
	Description: "clear any prior registration (tolerant of \"not found\"); " +
		"if the following registration action fails or is interrupted, no " +
		"claude-code registration remains — re-run --apply to recover",
}
```
Author the plugin argv the SAME way: package-level `Action`/argv-slice values or small builder functions living in `claudecode.go`/`codex.go` respectively, never in a runtime-agnostic file. Recommended argv (RESEARCH.md Code Examples, live-verified this session):
- Claude Code marketplace add: `claude plugin marketplace add seanb4t/engram`
- Claude Code install: `claude plugin install engram@engram --json -y`
- Claude Code update: `claude plugin update engram@engram --json -y`
- Codex marketplace add: `codex plugin marketplace add seanb4t/engram --json`
- Codex install: `codex plugin add engram@engram --json`
- Codex update (D-02, no native `update`): `codex plugin remove engram@engram --json` then `codex plugin add engram@engram --json`

**Up-front capability decline pattern** (`internal/setup/codex.go:87-96`, Codex's header decline — model for a probe-failure short-circuit):
```go
func (codexRuntime) Plan(env Environment, opts Options) (Plan, error) {
	if len(opts.Headers) > 0 {
		...
		return Plan{}, fmt.Errorf("codex: custom header(s) %s: ...: %w",
			strings.Join(names, ", "), ErrHeaderUnsupported)
	}
	...
}
```
This shows the established idiom for "this runtime's CLI cannot do X" as a typed sentinel error (`errors.Is`-checkable) returned early from `Plan()` — follow it if the plugin lane's absent-probe case needs a comparable typed signal (though D-12 explicitly wants "never a failed row" for a probe failure, so prefer a facet-level string over a `Plan()`-returned error for THAT specific case — see Shared Patterns, Pitfall 6).

**Probe-authoring pattern** (`internal/setup/codex.go:108`, `claudecode.go:116,165` doc comment): every `Plan()` already authors `Probe []string` in the SAME call that authors the write `Action`s — the plugin capability+state probe (`<cli> plugin list --json`) should be authored the identical way, in each runtime's own file, never centrally.

**Doc-comment discipline:** both files carry extensive doc comments explaining WHY (not just what) each branch exists, citing the specific D-xx decision it satisfies and the specific pitfall it avoids. New plugin code in these files should carry the same density of citation back to 03-CONTEXT.md's D-01..D-12.

---

### `internal/setup/apply.go` (MODIFIED) — service, request-response

**Analog:** itself — `execute()`'s 9-step sequence (`internal/setup/apply.go:186-227`, doc comment) is explicitly flagged by RESEARCH.md's Summary as the WRONG model to fold plugin logic into (a blind pre/post byte-compare cannot express "parse a version field and classify outdated/current"). Do not extend `execute()` itself; instead, add a PARALLEL function in this file (or in `plugin.go`) that reuses `runSeam`/`execTimeout`/`describeFailure`/`describeSeamError` — see the Anti-Pattern in Shared Patterns below.

**Read-then-decide-then-act shape to build**, modeled on `execute()`'s own probe→act→probe skeleton but branching on PARSED content rather than raw-byte-compare:
```go
// execute's pattern (apply.go:296-383), for STRUCTURE reference only —
// do not extend this function itself for plugin logic (see Anti-Pattern):
var probe1 RunResult
var probe1Err error
if hasProbe {
	probe1, probe1Err = runSeam(ctx, env, binary, plan.Probe[1:])
}
...
probe2, probe2Err := runSeam(ctx, env, binary, plan.Probe[1:])
...
if probe1Err == nil && probe1.Stdout == probe2.Stdout && probe1.Stderr == probe2.Stderr {
	res.Outcome = OutcomeAlreadyCorrect
} else {
	res.Outcome = OutcomeWrote
}
```
The plugin lane's own shape: ONE probe (`plugin list --json`) → parse → classify absent/outdated/current (or unavailable on parse/exit failure) → decide zero-or-more actions → run them through `runSeam` → done (no second probe needed for D-01's classification, since the FIRST probe's parsed content is already the classification input — unlike registration's opaque byte-compare, which needs two reads).

---

### `internal/skills/environment.go` (MODIFIED) — utility, file-I/O

**Analog:** itself — the existing three func-field seam.

**Seam-field pattern to extend** (`internal/skills/environment.go:22-44`):
```go
type Environment struct {
	ReadFile func(name string) ([]byte, error)
	WriteFile func(name string, data []byte, perm os.FileMode) error
	MkdirAll func(name string, perm os.FileMode) error
}

var OSEnvironment = Environment{
	ReadFile:  os.ReadFile,
	WriteFile: os.WriteFile,
	MkdirAll:  os.MkdirAll,
}
```
Add a new field the SAME way: `Lstat func(name string) (os.FileInfo, error)` mirroring `os.Lstat`'s signature, with `OSEnvironment.Lstat = os.Lstat` for production — a purely additive struct-field change (Pitfall 4 in RESEARCH.md). Zero impact on `Install`'s existing behavior since `Install` itself never calls it; this is a report-only seam for D-08's symlink-vs-copy check, consumed from `cmd/engram/setup.go` or a new small function in `internal/skills` (e.g. `skills.DetectNativePresence` or similar) that `Install` does not call.

---

### `internal/skills/install.go` (MODIFIED — D-08 report) — service, file-I/O

**Analog:** itself — `installFiles`/`installAgentsMDIndex`'s "walk skills, stat destinations, classify" idiom (`internal/skills/install.go:87-125`).

**Read-and-classify-without-writing pattern to add** (model on `installFiles`'s stat-then-classify loop, but READ-ONLY — never call `WriteFile`/`MkdirAll`):
```go
// installFiles (existing, install.go:97-125) — model for the STRUCTURE
// of "walk skill dirs, stat each, classify": a NEW D-08 report-only
// function should walk target.Dir similarly but call env.Lstat instead
// of env.ReadFile/WriteFile, classifying symlink-vs-regular per entry
// and counting files present, never touching WriteFile/MkdirAll at all.
```
Return a small report struct (path, count, `isSymlink bool` or similar) that `cmd/engram/setup.go` renders into the D-08 example text: `native: 5 skills present at ~/.agents/skills (symlink) — remove manually to avoid duplicates`.

---

### `cmd/engram/setup.go` (MODIFIED) — controller, request-response

**Analog:** itself — `setupApplySkillsFacet` is the EXACT pattern to mirror for the new plugin facet.

**Facet-composition pattern** (`cmd/engram/setup.go:130-182`, read verbatim this session):
```go
func setupApplySkillsFacet(row *setupRuntimeRow, registrationOutcome setup.Outcome, planTarget setup.SkillTarget, mutate bool, includeContent bool) setup.Outcome {
	row.Registration = string(registrationOutcome)
	target, targetErr := setupSkillsTarget(planTarget)
	if targetErr != nil {
		row.Skills = string(setup.OutcomeFailed)
		row.Reason = setupJoinReason(row.Reason, fmt.Sprintf("skills: %s: %v", row.Name, targetErr))
		return setup.AggregateOutcome(registrationOutcome, setup.OutcomeFailed)
	}
	...
	return setup.AggregateOutcome(registrationOutcome, skillsOutcome)
}
```
Author `setupApplyPluginFacet(row *setupRuntimeRow, registrationOutcome setup.Outcome, rt setup.Runtime, opts setup.Options, mutate bool) setup.Outcome` the IDENTICAL way: independently compute the plugin facet's own outcome, write its own scalar fields directly onto `row`, fold via `setup.AggregateOutcome`, call it from `setupRuntimeRowFromResult` immediately alongside the existing skills-facet call (`setup.go:602`). `AggregateOutcome` is documented symmetric/associative (`aggregate.go:29-56`), so a three-way fold (`AggregateOutcome(AggregateOutcome(reg, plugin), skills)`) is safe.

**Reason-joining pattern** (`cmd/engram/setup.go:103-113`):
```go
func setupJoinReason(existing, next string) string {
	if existing == "" {
		return next
	}
	return existing + "; " + next
}
```
Reuse this verbatim for folding a plugin-facet failure into `row.Reason` alongside (never instead of) a registration failure.

**Flat-scalar row-field pattern** (`cmd/engram/setup.go:356-376`, `setupRuntimeRow` struct — the SEVEN skills-facet fields are the template):
```go
Registration  string `json:"registration,omitempty"`
Skills        string `json:"skills,omitempty"`
SkillsDest    string `json:"skills_dest,omitempty"`
SkillsIndex   string `json:"skills_index,omitempty"`
SkillsDigest  string `json:"skills_digest,omitempty"`
SkillsBytes   string `json:"skills_bytes,omitempty"`
SkillsContent string `json:"skills_content,omitempty"`
```
The plugin facet's new fields MUST follow this exact shape: every field a plain `string` (never a struct/map/slice — `TestOperatorViewFixturesHaveNoUnsanitizedNesting` structurally forbids it, see Shared Patterns), `omitempty`-tagged, named with a `Plugin`-prefixed convention mirroring `Skills*` (e.g. `Plugin`, `PluginVersion`, `PluginSource`, `PluginNote` — exact names are plan-time discretion per CONTEXT.md, but the SHAPE is locked by this precedent).

**Summary/digest-string idiom** (`cmd/engram/setup.go:70-101`, `setupSkillsDigestSummary`/`setupHeadersSummary`): both render a slice/map into ONE comma-joined string for a row field — reuse this idiom if the plugin facet needs to render e.g. a marketplace source string or a note list.

**Exit-taxonomy note:** no new `Outcome` value should be introduced for "unavailable" (see Shared Patterns, Pitfall 6) — model D-12's "never a failed row" requirement on `SkillsOutcome`'s own precedence design (`internal/setup/aggregate.go:58-95`), which already has a clean "format==None always yields WouldWrite, never touching mutate" branch as the template for "capability probe failed → this facet contributes nothing that should ever fail the row."

---

### `cmd/engram/setup_test.go` (MODIFIED) — test, request-response

**Analog:** itself — `fakeSetupEnv`/`fakeSetupEnvWithRun`/`fakeSkillsEnv`/`withFakeSetupEnv` (`cmd/engram/setup_test.go:23-110`, read verbatim this session).

```go
func fakeSetupEnvWithRun(run func(context.Context, string, []string) (setup.RunResult, error), present ...string) setup.Environment {
	env := fakeSetupEnv(present...)
	env.Run = run
	return env
}
```
A plugin test scripts a `plugin list --json` response DISTINCTLY from the registration probe's response, keyed on the `args` slice's content (e.g. `args[1] == "plugin"` vs `args[1] == "mcp"`) inside the same closure signature — no new test-environment type needed, since `Environment.Run`'s signature (`ctx, path, args`) already carries everything a plugin-vs-registration dispatch needs.

`fakeSkillsEnv()` (`cmd/engram/setup_test.go:67-83`) is the reusable in-memory `skills.Environment` fake — extend it with an `Lstat` closure (a map-backed fake symlink registry) for D-08 tests, per RESEARCH.md's Wave 0 Gaps list.

---

### `cmd/engram/operator_view_setup_test.go` (MODIFIED — fixtures) — test, request-response

**Analog:** itself — `setupViewFixtures()` (`cmd/engram/operator_view_setup_test.go:22-115`, read verbatim this session). NOTE: RESEARCH.md's own text names `operator_output_test.go` as the extension point, but the actual fixture function `setupViewFixtures()` (merged into `operatorViewFixtures()` in `operator_output_test.go:170`) lives in this SIBLING file, per the file's own header comment ("this plan's (02-01) setup command fixtures... merged into operatorViewFixtures()"). Extend `operator_view_setup_test.go`, not `operator_output_test.go` itself.

```go
headerGateway := setupRuntimeRow{
	Name:    "claude-code",
	Present: true,
	Outcome: "would-write",
	Command: "claude mcp remove engram --scope user; claude mcp add ...",
	Headers: "CF-Access-Client-Id=CF_ID,x-gateway-api-key=GATEWAY_KEY",
}
```
Add a plugin-facet fixture the SAME way: a `setupRuntimeRow` literal with the new flat plugin fields populated, appended into the `map[string][]any{"setup": {...}}` return — this is what proves `TestOperatorViewFixturesHaveNoUnsanitizedNesting` stays green with the new fields (Pitfall 6/REQ-plugin-facet-reported's fixture requirement).

---

### `skill/engram/.codex-plugin/plugin.json` (NEW) — config, file-I/O

**Analog:** `skill/engram/.claude-plugin/plugin.json` (read verbatim this session — the WHOLE file, 5 lines):
```json
{
  "name": "engram",
  "version": "0.16.1",
  "description": "Self-hosted, correctable, OAuth-secured memory for coding agents: session-start recall, curation discipline, and a two-tier per-workspace memory scope. Register the engram MCP server with /engram-setup."
}
```
Mirror field-for-field (`name`, `version`, `description` — identity fields the drift gate compares byte-for-byte); RESEARCH.md's live-fetched `agent-plugins.org` schema confirms only `["$schema","name"]` are required, `interface` is optional, and `.claude-plugin/plugin.json`'s own omission of `author`/`interface` is independent evidence Anthropic's own loader accepts a 3-field manifest — ship the same minimal shape as the primary attempt (flag Codex's `interface` requirement as an open question / post-ship verification per RESEARCH.md Open Question 2, not a blocker).

**Vendor-neutrality constraint** (`skill/engram/hooks/tests/test_no_residual_memory_oauth.py:1-52`, read verbatim this session): this new file lands under `skill/engram/` and is therefore in-scope for `test_no_residual_old_branding`/`test_no_private_hosts_in_shipped_bundle`'s whole-bundle scan (only `hooks/tests/` is excluded) — its `description` must stay vendor-neutral exactly like `.claude-plugin/plugin.json`'s own description already is; do not introduce `fzymgc`/`litellm`/pre-rebrand identifiers.

---

### `release-please-config.json` (MODIFIED) — config, batch

**Analog:** itself — the existing `.claude-plugin/plugin.json` sync entry (read verbatim this session):
```json
{
  "type": "json",
  "path": "skill/engram/.claude-plugin/plugin.json",
  "jsonpath": "$.version"
}
```
Duplicate this exact entry shape for `skill/engram/.codex-plugin/plugin.json`, appended to the SAME `extra-files` array (`packages["."].extra-files`), so release-please keeps both manifests' `$.version` in sync on every release.

---

### `internal/setupgen/setupgen.go` (MODIFIED, or `_test.go`) — service, transform

**Analog:** itself — `Cases()`, `Render()`'s `adds`-filter (`internal/setupgen/setupgen.go:42-117`, read verbatim this session).

**The exactly-one-match filter to extend (Pitfall 7)**:
```go
var adds []setup.Action
for _, action := range plan.Actions {
	if len(action.Args) >= 3 && action.Args[0] == "claude" && action.Args[1] == "mcp" && action.Args[2] == "add" {
		adds = append(adds, action)
	}
}
if len(adds) != 1 {
	return "", fmt.Errorf("setupgen: %s: expected exactly one claude mcp add action, got %d", c.Label, len(adds))
}
```
If plugin actions ARE authored into `Plan.Actions` (Open Question 1), add a SECOND filter loop matching `{"claude","plugin",...}`-shaped actions with its OWN count assertion — the existing filter's `mcp add`-specific match will NOT catch a new action kind silently (this is Pitfall 7's exact warning). If the planner instead resolves Open Question 1 by keeping plugin actions ENTIRELY outside `Plan.Actions` (the leading candidate per RESEARCH.md's own recommendation), this file needs NO change beyond, at most, a hand-authored prose sentence outside the generated region of `skill/engram/commands/engram-setup.md` — confirm which resolution was chosen before touching this file.

**Manifest identity drift test location** (new, per REQ-codex-plugin-manifest): follow `setupgen_test.go`'s existing table-test style (not shown here, but same package/testing idiom as `leafpurity_test.go`'s file-scan pattern) for a new `TestPluginManifestIdentityMatches` comparing `skill/engram/.claude-plugin/plugin.json`'s and `skill/engram/.codex-plugin/plugin.json`'s `name`/`version`/`description` fields via `encoding/json` unmarshal + struct-field comparison — this test can live in `internal/setupgen` (co-located with the other generation/drift gates) or a small sibling package; either is consistent with the Architectural Responsibility Map in RESEARCH.md.

---

## Shared Patterns

### Stdlib-only leaf gate (`internal/setup`)
**Source:** `internal/setup/leafpurity_test.go:81-134` (verbatim, read this session)
**Apply to:** `internal/setup/plugin.go`, `internal/setup/pluginversion.go`, and any other new/modified non-test `.go` file directly inside `internal/setup`
```go
firstSeg, _, _ := strings.Cut(importPath, "/")
if strings.Contains(firstSeg, ".") {
	nonStdlib = append(nonStdlib, offender{path, importPath})
}
```
`golang.org/x/mod/semver`'s first import-path segment is `golang.org` — contains a `.` — and FAILS this build-time gate the instant it is imported from any non-test file under `internal/setup`. The version comparator MUST be a small, local, stdlib-only implementation (`regexp` + `strconv`, following `cmd/engram/buildversion.go`'s technique) — never `golang.org/x/mod/semver` imported directly into this package. `TestSetupPackageIsStdlibOnlyLeaf` also forbids importing anything from this module (`github.com/seanb4t/engram/...`) other than nothing — `internal/setup` must never import `cmd/engram`, `internal/config`, `internal/skills`, etc.

### Facet composition in `cmd/engram`, never in the shared executor
**Source:** `cmd/engram/setup.go:130-182` (`setupApplySkillsFacet`)
**Apply to:** the new plugin facet in `cmd/engram/setup.go`
Compute each facet's outcome independently, write its own flat scalar fields directly onto the row struct, fold outcomes via `setup.AggregateOutcome` — never teach `internal/setup/apply.go`'s shared `execute()` about a specific facet's content-aware classification logic (see Anti-Pattern below).

### Anti-Pattern: folding content-aware classification into the blind byte-compare executor
**Source:** RESEARCH.md Summary + Anti-Patterns section; `internal/setup/apply.go:186-227` (`execute()`'s own doc comment, steps 1-9)
**Do not:** extend `execute()`'s pre/post byte-compare (`probe1.Stdout == probe2.Stdout && probe1.Stderr == probe2.Stderr`) to classify plugin state. D-01's "outdated" classification is a SEMANTIC comparison (parsed version fields) that this blind comparator cannot express — folding it in would either misclassify every "current" plugin as `wrote` on every re-run (violating REQ-plugin-install-or-update's "does nothing when already current"), or require special-casing the executor by content, which `apply.go`'s own doc comments forbid.

### Flat-scalar-only row fields (no nesting)
**Source:** `cmd/engram/setup.go:306-376` (`setupRuntimeRow` struct's own extensive doc comment, explicitly citing `TestOperatorViewFixturesHaveNoUnsanitizedNesting`)
**Apply to:** every new plugin-facet field on `setupRuntimeRow`
Every field must be a plain `string` (never `json.RawMessage`, a map, or a slice) — `sanitizeViewValue`'s scalar-only sanitizing branch (`cmd/engram/operator_view.go`) is bypassed by any non-scalar kind, and `TestOperatorViewFixturesHaveNoUnsanitizedNesting` (`cmd/engram/operator_output_test.go`) fails on exactly that shape by design. Render a list (e.g. multiple notes) as one comma/semicolon-joined string, following `setupSkillsDigestSummary`/`setupHeadersSummary`'s existing idiom.

### AggregateOutcome's five-value exhaustive vocabulary
**Source:** `internal/setup/aggregate.go:6-56`, `internal/setup/exit.go:26-74`
**Apply to:** the plugin facet's outcome contribution
Model the plugin facet's outcome as one of the FIVE existing `Outcome` values (`OutcomeFailed`, `OutcomeWrote`, `OutcomeAlreadyCorrect`, `OutcomeWouldWrite`, `OutcomeNotPresent`) — do not introduce a sixth `Outcome` constant for "unavailable." Per RESEARCH.md Pitfall 6/Assumption A5: model `unavailable: <reason>` (D-12) as a STRING value on a dedicated plugin-facet field, with the facet's AGGREGATE-able outcome contributing `OutcomeAlreadyCorrect` or `OutcomeWouldWrite` (never `OutcomeFailed`) so D-12's "never a failed row" requirement holds by construction. If a genuinely new `Outcome` value turns out to be unavoidable, it must be added to THREE sites in the same commit: `aggregate.go`'s `precedenceOrder`/`isRecognizedOutcome`, and `exit.go`'s `Classify` switch — `TestClassifyExhaustiveOutcomeCombinations` (exit_test.go) is the mechanical proof this was done.

### Injectable-seam-per-package testing discipline (repo rule `m45p2b4bp7`)
**Source:** `internal/setup/environment.go:14-73`, `internal/skills/environment.go:8-53`, `cmd/engram/setup_test.go:23-110`
**Apply to:** every new exec/filesystem touch point this phase adds
Every external-boundary call (subprocess exec, file read/write/stat) goes through a struct-of-func-fields `Environment` seam, never a direct `os.*`/`exec.*` call from production logic — and every test drives a FAKE `Environment`, never the real machine's PATH or home directory. `Lstat`'s addition to `internal/skills.Environment` (D-08) and any new field on `internal/setup.Environment` (if the plugin lane needs one beyond what already exists — `LookPath`/`Getenv`/`HomeDir`/`Run` already covers everything the RESEARCH.md's Open Question 3 discusses) must follow this exact `OSEnvironment`-plus-fake shape.

### SPDX header (verbatim, every new Go file)
**Source:** any tracked `.go` file in this repo, e.g. `internal/setup/plan.go:1-2`
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
```
Applies to every new `.go` file this phase creates (`plugin.go`, `plugin_test.go`, `pluginversion.go` if split out, `pluginversion_test.go`). Do NOT add this header to `skill/engram/.codex-plugin/plugin.json` (JSON has no comment syntax, and `.licenserc.yaml` scope excludes non-Go/Markdown shipped manifests the same way it excludes `.claude-plugin/plugin.json`) or to `skill/engram/commands/engram-setup.md` (excluded, per CLAUDE.md's own list, as slash-command markdown).

## No Analog Found

None — every file in CONTEXT.md's/RESEARCH.md's expected-files list has a strong, tracked, same-role analog in the existing codebase (this phase is explicitly an EXTENSION of an established per-runtime Plan/Apply/facet architecture, not a new architectural pattern).

## Metadata

**Analog search scope:** `internal/setup/`, `internal/skills/`, `internal/setupgen/`, `cmd/engram/setup*.go`, `cmd/engram/operator_view*.go`, `cmd/engram/buildversion.go`, `skill/engram/.claude-plugin/`, `release-please-config.json`.
**Files scanned:** 22 (all read verbatim this session; all confirmed git-tracked via `git ls-files`).
**Pattern extraction date:** 2026-09-14
