# Architecture Research: `engram setup` v2 — Plugin Delivery, Custom Headers, Drift Detection, Completions

**Domain:** Go CLI subcommand extension — widening an already-shipped `internal/setup` writer abstraction, never replacing it.
**Milestone:** `2026-09-13.01` — Setup v2.
**Researched:** 2026-09-13
**Confidence:** HIGH for structural integration points (every claim below cites a real `file:line` read in this pass). MEDIUM for exact third-party CLI syntax not yet live-verified this milestone (`claude plugin`, `codex plugin`, codex's arbitrary-header capability) — flagged explicitly, never guessed, following this repo's own established discipline (see prior `.planning/research/ARCHITECTURE.md`, §"Sources").

## 0. What already exists (read first, do not rebuild)

`internal/setup` (package doc, `internal/setup/plan.go:1-17`) already ships the whole writer abstraction the prior milestone's research proposed: a `Runtime` interface (`internal/setup/runtime.go:38-54`) with four registered implementations — `ClaudeCode`, `Codex`, `OpenCode`, `Generic` (`internal/setup/runtime.go:70`) — a five-value `Outcome` enum (`internal/setup/plan.go:32-55`), a `Plan`/`Action`/`Result` triple (`internal/setup/plan.go:93-252`), a shared `execute()` executor that is the ONLY place any process gets run (`internal/setup/apply.go:213-369`), and a `SkillFormat`/`SkillTarget` pair (`internal/setup/plan.go:57-91`) that already threads skills delivery through the same Plan a runtime authors for MCP registration. `cmd/engram/setup.go` is the thin cobra entrypoint: it resolves flags into `setup.Options` (`setupResolve`, `cmd/engram/setup.go:318-368`), builds one report row per runtime via `setup.Preview`/`setup.Apply` (`setupBuildRows`/`setupApplyRun`, `cmd/engram/setup.go:283-289,511-546`), and folds each runtime's registration facet with its skills facet through `setup.AggregateOutcome` (`internal/setup/aggregate.go:43-56`) into ONE outcome per row (`setupRuntimeRowFromResult`, `cmd/engram/setup.go:437-454`). `internal/setupgen` already generates the `/engram-setup` fallback/delegation tables FROM `setup.ClaudeCode.Plan` (`internal/setupgen/setupgen.go:136-179`), diffed against the checked-in `skill/engram/commands/engram-setup.md` region by a CI gate (`Check`/`compare`, same file lines 136-163). None of this is new — it is the substrate every feature below extends.

Everything this milestone adds is a widening of THIS substrate, not a parallel structure. The governing discipline already encoded in the package (worth restating because every recommendation below depends on it): every runtime-specific string is **authored in that runtime's own file** (`internal/setup/plan.go:11-16`'s package doc, "AUTHORED HERE and nowhere else"); the shared executor never special-cases a runtime by name; an unrecognized enum value is a **hard error**, never a silent default (`setup.SkillFormat`'s doc comment, `plan.go:57-66`, and `cmd/engram/setup.go:54-64`'s `setupSkillsTarget` switch, which has NO `default:` fallback case — an authoring bug surfaces as an error naming the unknown value).

## 1. Plugin-first delivery — where it fits

### It is not a new Action kind

`Action` (`internal/setup/plan.go:93-116`) is already a generic `{Args []string; Tolerant bool; Description string}` — argv, a tolerance flag, and a label. Claude Code's Plan already authors TWO actions in sequence today (`claudeCodeRemoveAction` then `mcp add`, `internal/setup/claudecode.go:62-172`), executed by the same shared loop with no per-action type discrimination (`apply.go:310-345`). A `claude plugin marketplace add ...` / `claude plugin install|update engram@...` step is **structurally identical** to the existing tolerant-remove-then-fatal-add shape — it is one or two more `Action` values appended to the SAME `Plan.Actions` slice, in `claudecode.go`'s own `Plan()`, authored alongside the `mcp add` action it already authors. **No new field on `Action`, no new "kind" enum, no change to `execute()`** — the shared executor already runs an arbitrary ordered sequence of tolerant/fatal argv actions and reports the first fatal failure while preserving prior tolerant-step notes (`apply.go:309-345`). This is the cheapest and most consistent place to land plugin management, and it is the one place the package's own AUTHORED-HERE discipline already expects new runtime behavior to appear.

Two real unknowns to verify live before locking the exact `Action.Args`, flagged rather than guessed (mirroring the prior research pass's treatment of unverified CLI surfaces):
- Whether `claude plugin install`/`codex plugin` refuse-on-already-installed (like `claude mcp add`, `claudecode.go:33-39`) or silently overwrite/update (like `codex mcp add`, `codex.go:39-41`) — this decides whether the plugin step needs its own tolerant-then-fatal pair or a single idempotent call.
- Whether `codex plugin` exists at all as a stable, documented surface (the milestone context asserts it does; this was not independently re-verified in this pass — treat as MEDIUM confidence pending a live check, same standard the prior research applied to codex/opencode's `mcp add` surface before it was live-verified).

### The skills-skip signal: extend `SkillFormat`, not `Outcome`

The milestone requires that when the plugin path is used, `internal/skills.Install` must **never run** for that runtime — the plugin already carries `skill/engram/skills/*` byte-identical (vendored via `task skills:vendor` against the same source tree, per `PROJECT.md`'s "What shipped" bullet on skills distribution). Today `cmd/engram/setup.go:106-158`'s `setupApplySkillsFacet` unconditionally calls `skills.Inventory()`/`skills.Install()` for every present runtime whose `SkillTarget.Format` is not `skills.FormatNone`. The correct extension point is a **new, fourth `SkillFormat` value** — e.g. `SkillFormatPlugin` — added to `internal/setup/plan.go:69-79`'s const block, alongside `SkillFormatNone`/`SkillFormatNative`/`SkillFormatAgentsMD`. `cmd/engram/setup.go:54-64`'s `setupSkillsTarget` switch gets one more `case`, mapping `SkillFormatPlugin` to a **new no-op path that never calls `skills.Install`** at all (structurally the same as `FormatNone`'s already-no-op branch in `internal/skills/install.go:72-74`, but semantically distinct — "no destination" vs. "someone else's job" — so it must NOT collapse to `skills.FormatNone` itself, or a future reader loses the reason). This satisfies "no default case ever maps an unrecognized value to a destination nobody authored" (the existing comment at `cmd/engram/setup.go:47-53`) by construction: `SkillFormatPlugin` is a real, named, explicitly-handled value, not a fallback.

`setup.SkillsOutcome` (`internal/setup/aggregate.go:78-95`) already special-cases `format == SkillFormatNone` as a permanent `OutcomeWouldWrite`. It needs a parallel arm for `SkillFormatPlugin` — but with a DIFFERENT verdict: a plugin-delivered runtime's skills facet should read as `OutcomeAlreadyCorrect`-shaped once the plugin action itself succeeds (there is nothing further to converge on the skills side — the plugin call already did the whole job), not `OutcomeWouldWrite` forever. The cleanest reading: **fold the plugin's own install/update Outcome directly into what was previously the "skills" facet slot** — i.e., for a plugin-delivery runtime, `row.Skills` (or a renamed/adjacent field) reports the PLUGIN action's own classified outcome (would-write / wrote / already-correct / failed, exactly as computed by the shared executor for that action), not a separate `skills.Install` report. This keeps `AggregateOutcome`'s existing two-facet fold (`cmd/engram/setup.go:451`, `AggregateOutcome(registrationOutcome, skillsOutcome)`) working **unchanged** — the "skills facet" is just sourced differently depending on `SkillFormat`, and the fold itself needs no new arity. (A three-facet fold — registration + plugin + skills — is only needed if a future runtime needs BOTH a plugin AND a filesystem skills write simultaneously; nothing in this milestone requires that, so do not build it speculatively.)

### Report rows: additive fields only

`setupRuntimeRow` (`cmd/engram/setup.go:235-254`) already carries seven skills-facet fields (`Skills`, `SkillsDest`, `SkillsIndex`, `SkillsDigest`, `SkillsBytes`, `SkillsContent`) as plain `string` fields with `omitempty` JSON tags — the exact shape a plugin facet needs. Add parallel fields, e.g. `Plugin string \`json:"plugin,omitempty"\`` (the plugin action's classified outcome, reusing the `Outcome` string vocabulary verbatim — no new type) and optionally `PluginVersion`/`PluginMarketplace` detail strings for `--output json`. This is **purely additive to the JSON schema** — every existing field stays present and unchanged, satisfying "extend without breaking the shipped vocabulary" literally: an old consumer parsing today's JSON still finds every field it expects; a new consumer additionally finds `plugin`. No renderer code is needed for any of this — `renderOperator`'s `viewFields` walks the marshaled struct generically (the same "falls out for free" property the prior milestone's research already established and this milestone's own code comments restate, `cmd/engram/setup.go:224-234`).

### Interaction with drift detection's probe model

Plugin state needs its own read-verb for the shared executor's before/after byte-compare (D-08, `apply.go:107-119,363-367`) to classify `wrote` vs. `already-correct` honestly for the plugin facet, exactly as `Plan.Probe` already does for MCP registration. Today `Plan.Probe` is a single `[]string` (`plan.go:136-147`) — ONE probe per runtime. Once a runtime plans BOTH an MCP-registration probe and a plugin-state probe, the field needs to widen. Rather than inventing a second flat field, generalize to a **small, named slice** at this point — see §4, which needs the identical generalization for header/URL drift comparison. Do this widening ONCE, for both features together, not twice.

## 2. `setupgen` and the generated `/engram-setup` command

`internal/setupgen/setupgen.go:24-46`'s `PlanFunc`/`Case`/`Cases()` already drive four synthetic `setup.Options` (oauth, oauth-client, bearer, none) through `setup.ClaudeCode.Plan` and render two tables: a "Claude Code fallback registration" table and a "Delegation preview" table (`Render`, lines 50-91). The fallback table is built by filtering `plan.Actions` for the ONE action shaped `{"claude","mcp","add",...}` (lines 78-86) and **hard-failing if it does not find EXACTLY ONE such action** (`if len(adds) != 1 { return "", fmt.Errorf(...) }`). Once `claudecode.go`'s `Plan()` starts appending plugin-management actions to the SAME `Plan.Actions` slice (§1), this filter still finds exactly one `mcp add` action among several — **it will not break by itself**. But it will also not RENDER the new plugin actions, which is precisely the gap the milestone's own framing warns about ("the CI drift gate will fail otherwise") — not because the existing gate breaks mechanically, but because the generated prose would then be **incomplete relative to what `--apply` now does**, which is exactly the class of drift `setupgen` exists to prevent (see the prior milestone's research, §3, "Two-paths-must-agree").

Concrete change to `internal/setupgen/setupgen.go`:
- Add a second filter loop alongside the existing `adds` loop (lines 78-83), matching actions shaped `{"claude","plugin",...}`, and assert a count invariant appropriate to however many plugin actions `Plan()` ends up authoring (one or two, per §1's open question).
- Add a third rendered section (e.g. `### Claude Code plugin installation`) to `Render`'s output (after line 90's `fallback.String()`), using the same `commandCell` escaping helper (lines 93-104) — no new escaping logic needed.
- `readRegion`/`compare`/`Check`/`Write` (lines 106-179) are **generic over the whole rendered blob** — they byte-compare the ENTIRE region against a fresh `Render()` call and do not know or care how many tables are inside it. **No change needed to the drift-detection mechanism itself** — only to what `Render()` produces. Once `Render()`'s shape changes, the checked-in `skill/engram/commands/engram-setup.md` region (regenerated via `task surfaces:gen` per the prior milestone's established workflow) is what must be regenerated and committed in the SAME change — this is the literal mechanism by which "the CI drift gate will fail otherwise" cashes out: it is a feature of the design working as intended, not a defect to route around.
- `Cases()` (lines 34-46) does not need a new case for plugin delivery itself — plugin actions are auth-mode-independent (every one of the four synthetic auth options should author the identical plugin-management actions), so the SAME four cases already exercise it. `Cases()` DOES need a new case once custom headers (§3) introduce a mode distinct from `bearer` that the fallback table should document — add a fifth `Case` entry there, following the exact pattern the existing four already use (lines 36-44).

## 3. Custom headers as "bearer generalized," not a new top-level mode

### Current shape

`Options` (`internal/setup/runtime.go:18-30`) carries a bare `Auth string` (one of `oauth|oauth-client|bearer|none`, validated by `config.ValidateSetupAuth`, `cmd/engram/setup.go:330-341`) plus `TokenFile string`, whose ONLY effect today is (a) a literal-vs-reference choice for the `generic` pseudo-runtime's placeholder text (`bearerProvenance`, `internal/setup/plan.go:254-277`) and (b) a `token_file=ignored` marker on every native runtime's report row (`tokenFileIgnoredMarker`, `internal/setup/apply.go:41-52,266-281`). Every native runtime's `bearer` case hard-codes ONE header: `Authorization: Bearer ${ENGRAM_TOKEN}` in claude-code's own dialect (`claudecode.go:161-163`), `--bearer-token-env-var ENGRAM_TOKEN` in codex's OWN flag (`codex.go:106-111` — codex has NO generic `--header` flag at all, only this one fixed shape), and `Authorization=Bearer {env:ENGRAM_TOKEN}` in opencode's `KEY=VALUE` dialect (`opencode.go:126-135`).

### The gap this milestone must close

A gateway header like LiteLLM's `x-litellm-api-key` cannot be named today — `bearer` is a single, fixed header name AND value shape, wired per-runtime, with no way to supply an arbitrary header. The fix must (1) keep the secret-as-env-var-reference-only invariant every runtime's bearer path already honors, (2) not change `--auth`'s four existing accepted values or their help text, and (3) explicitly account for codex's narrower CLI (no generic `--header`, only the one fixed `--bearer-token-env-var` flag).

### Recommended model

Add a new field, `Options.Headers []HeaderSpec`, where `HeaderSpec{Name string; EnvVar string}` — `Name` is the literal HTTP header name (`Authorization`, `x-litellm-api-key`, ...), `EnvVar` is an environment-variable NAME (never dereferenced by engram — no `os.Getenv` call anywhere in this package today except opencode's unrelated `XDG_CONFIG_HOME` read, `opencode.go:154-163`, and this must stay that way). This is additive to `Options` — nothing existing is removed or renamed.

- **`bearer` stays byte-identical.** Do not collapse `bearer` into the new header vocabulary at the CLI-flag or help-text layer — `setupResolve` (`cmd/engram/setup.go:318-368`) keeps accepting exactly `oauth|oauth-client|bearer|none` with unchanged validation and unchanged `setupLongDescription` prose (`cmd/engram/setup.go:562-611`), satisfying "without breaking existing tests and help text" literally. Internally, `bearer`'s existing per-runtime `case "bearer":` arms can OPTIONALLY be rewritten to synthesize `[]HeaderSpec{{Name: "Authorization", EnvVar: "ENGRAM_TOKEN"}}` and call a new shared per-runtime header-rendering helper — but this is a refactor for code reuse, not a behavior change, and is not required for correctness; the safer, lower-risk path is to leave every existing `bearer` arm untouched and add headers as a genuinely parallel case.
- **A new `--auth header` mode** (exact name a planning decision — `header`, `custom-header`, and `bearer-custom` are all defensible) requires a new, repeatable `--header NAME=ENVVAR` flag (cobra `StringArray`, mirroring the existing `--client-id` requiredness-gating pattern at `cmd/engram/setup.go:343-349`: required when `--auth header`, rejected for every other mode). Each runtime's `Plan()` gets one more `case "header":` arm:
  - **claude-code**: repeat `--header "Name: ${ENVVAR}"` per `HeaderSpec` — its `--header` flag already accepts arbitrary text (live-verified for the bearer form, `claudecode.go:82-92`), so this generalizes directly.
  - **opencode**: repeat `--header "Name={env:ENVVAR}"` per `HeaderSpec` — same generalization, its `KEY=VALUE` dialect already supports any key (`opencode.go:44-60`).
  - **codex**: **a real capability gap, not a rendering detail.** Codex's only header-shaped flag is `--bearer-token-env-var`, fixed to `Authorization: Bearer <value>` (`codex.go:34-41`). A `--auth header` request naming any header OTHER than exactly `{Authorization, <one EnvVar>}` has **no expressible form on codex's CLI today** and must return `ErrAuthModeUnsupported` (the same sentinel opencode already returns for `oauth-client`, `opencode.go:35-41`) — reported as an explicit per-runtime failed row naming the mode, never silently dropped or downgraded. A request naming exactly one `Authorization` header CAN degrade to the existing `--bearer-token-env-var` call. This asymmetry is exactly the kind of "states plainly which are unsupported for that runtime" case `REQ-register-auth-modes` already covers for opencode/oauth-client, and it needs live re-verification against `codex mcp add --help` at implementation time (flagged, not guessed) before this arm is written.
  - **generic**: trivial — `genericMCPServer.Headers` is already `map[string]string` (`generic.go:58-62`); add every `HeaderSpec` to it, reusing `bearerProvenance`'s existing literal-vs-reference choice per entry (`generic.go:99-115` already documents this exact tradeoff for the one bearer header).
- **No new `Outcome` value.** Header authoring only changes what argv/config an action carries — it does not change the five-value classification vocabulary at all.
- **`Options.TokenFile`'s existing scope stays intact** — it remains meaningful only for `generic` and only for the pre-existing `bearer`/single-header case; it is not generalized to apply per-`HeaderSpec` (a multi-header set with per-header provenance files is out of scope unless a concrete need surfaces).

## 4. Drift detection + reconcile hand-edits — the hardest, riskiest slice

### Why today's probe model cannot answer this question

`Plan.Probe` (`plan.go:136-147`) and the shared executor's convergence check (`apply.go:213-369`, esp. 355-367) are **deliberately content-blind**: two raw reads are byte-compared, and the ONLY question answered is "did anything change" — never "what changed" or "should this difference be preserved." This is not an oversight; it is a load-bearing design choice stated repeatedly in the package's own comments (`apply.go:29-35`, "sanitizeViewValue strips only C0 controls... never string-matched to decide an Outcome"; `opencode.go:79-93`, an EXPLICIT prior decision to never parse `mcp list`'s table output because "that would buy a dependency on a third-party output format this design already rejected"). Real drift detection — "compare the full existing registration (URL, auth shape, header set) against what it would write" — requires INTERPRETING probe output, which is a genuinely new capability this package does not have anywhere today, for any runtime.

### What each runtime's probe actually gives you

| Runtime | Probe (`plan.go` field) | Format | Parseable safely? |
|---|---|---|---|
| claude-code | `claude mcp get engram` (`claudecode.go:94-100,134,152,166`) | Human-readable block (Type/URL/Headers/Scope) | Structured enough to line-scan, but format is third-party and undocumented as a stable contract |
| codex | `codex mcp get engram --json` (`codex.go:44-45,84`) | **JSON** | Yes — `encoding/json`, stdlib, zero new dependency; the best-case runtime |
| opencode | `opencode mcp list` (`opencode.go:62-93`) | Box-drawing human table, lists EVERY server, dials the network for each | **No** — the file's own doc comment (lines 68-81) already forbids parsing this; treating it as parseable would silently reopen a rejected decision |
| generic | none (`generic.go:76-88`, zero Actions, zero Probe) | N/A | N/A — nothing to observe; drift detection is meaningless for a pseudo-runtime that never writes |

This table is the crux of part (d): **the executor "only sees argv + stdout," and one of three real runtimes has already had output-parsing explicitly rejected for it.** Real drift detection cannot be uniform. It must be a per-runtime, opt-in capability that degrades safely (falls back to today's raw byte-compare, which already never falsely claims convergence) rather than a shared parser the executor applies blindly.

### Recommended shape

1. **A normalized `ObservedRegistration` struct**, new file `internal/setup/observe.go`:
   ```go
   type ObservedRegistration struct {
       Parseable bool              // false => caller MUST fall back to raw byte-compare, never guess
       URL       string
       Headers   map[string]string // header NAME -> value AS ECHOED (still the literal "${ENGRAM_TOKEN}"-shaped reference text — mirrors D-05's already-observed behavior that a runtime's own `get`/`list` never resolves the secret)
       Raw       string            // always retained for Reason/Notes and as the fallback compare basis
   }
   ```
2. **One parser per runtime, authored in that runtime's own file** — `parseClaudeCodeRegistration` in `claudecode.go`, `parseCodexRegistration` (via `encoding/json`, the easy case) in `codex.go` — mirroring the existing AUTHORED-HERE discipline for `Plan()`/`Probe` (`plan.go:11-16`). **opencode authors none** — its probe stays raw-compare-only, exactly as today, and this must be a conscious, documented omission, not a gap discovered later.
3. **One comparison function**, e.g. `compareRegistration(observed ObservedRegistration, plan Plan, opts Options) DriftReport`, living in ONE place (not scattered per-runtime) so the **"cannot reproduce → preserve" decision is centrally auditable** — the same discipline `AggregateOutcome`'s single declared `precedenceOrder` (`aggregate.go:10-16`) already models for outcome-folding. A field engram cannot prove wrong (an operator-added header it doesn't recognize, or ANY field on a non-`Parseable` runtime) classifies as **preserved**, never as drift.
4. **A new `Outcome` value is a real vocabulary widening, not a free addition.** `precedenceOrder` (`aggregate.go:10-16`), `isRecognizedOutcome`'s implicit exhaustiveness (same file), and `Classify`'s exhaustive switch (`exit.go:48-74`) all hard-code today's five values and explicitly treat anything unrecognized as failure. Adding `OutcomePreserved` means touching all three sites in one commit, plus `setupApplySummary`'s cmd/engram-side tally (`cmd/engram/setup.go:479-496`, currently `wrote`/`already`/`failed` only). Recommend `OutcomePreserved` sit in the precedence table between `OutcomeAlreadyCorrect` and `OutcomeWouldWrite` (a preserved hand-edit means "correctly wrote nothing," closer in spirit to already-correct than to a bare preview). This is the one part of the whole milestone that widens the shipped Outcome vocabulary — call it out explicitly in planning, unlike every other addition above, which stays purely additive to structs/fields.
5. **Reconciliation (actually merging a preserved header into the write) requires a genuinely new hook**, because `execute()` today runs probe #1 and then the write action UNCONDITIONALLY, never branching on probe content by design (`apply.go`'s repeated "report rather than diagnose" framing). Recommend an OPTIONAL, explicitly-named second-stage authoring step — e.g. `Runtime` gains an optional `Reconcile(observed ObservedRegistration, opts Options) (Plan, error)` a runtime may implement (an interface-assertion pattern exactly like `optInOnlyRuntime`, `runtime.go:72-84`) — called after probe #1, before the write, to let a runtime rewrite ITS OWN actions to append a preserved element. This is new surface, not a repurposing of D-11's failure-reporting discipline, and should be scoped and reviewed as its own decision, not folded silently into the executor's existing failure path.
6. **Given the size and risk here, split delivery**: ship **read-only drift reporting** first (report what differs and what would be preserved, `--output json` only, zero behavior change to what `--apply` writes) as an independently shippable slice; defer **actual write-time reconciliation** (the `Reconcile` hook, header-merging into the live write) as a following slice — matching this repo's own pattern of shipping the legible, non-destructive half of a feature before the mutating half (see the prior milestone's `spine-review scan`→`consolidate` sequencing, `PROJECT.md` v0.13.x section).

## 5. Shell completions + manpages — completions are already shipped

**Correction to the milestone framing:** shell completions are **not new work**. Cobra auto-registers `completion` (never disabled — no `CompletionOptions.DisableDefaultCmd` anywhere in `cmd/engram`, confirmed by search), and `cmd/engram/cmdwalk.go:23`'s `commandWalkSkip` predicate (`cmd.Hidden || cmd.Name() == "help" || cmd.Name() == "completion"`) explicitly excludes it from `buildCatalog`'s walk — proving it already exists as a live command deliberately kept out of the classified surface. More importantly, `.goreleaser.yaml`'s `homebrew_casks.hooks.post.install` **already calls it**: `completion = system_command binary, args: ["completion", shell]` for bash/zsh/fish, writing all three completion files, with a matching `hooks.post.uninstall` cleanup — shipped in the PRIOR milestone's Phase 1 (`.planning/milestones/2026-08-23.01-phases/01-version-homebrew-distribution/01-CONTEXT.md` D-09/D-10, `01-SUMMARY.md`). `PROJECT.md`'s "Carried tech debt" line bundling "shell completions / man pages via the cask" as still-open is imprecise — **only manpages remain.** Flag this to the roadmapper as a scope correction before phases are cut.

### Manpages — the real remaining work

`cobra/doc`'s `GenManTree` is NOT auto-registered like `completion` — it needs an explicit call site, and it is confirmed already an indirect dependency (per the milestone's own framing), so promoting it to direct in `go.mod` is metadata-only, identical in kind to the prior milestone's `go.yaml.in/yaml/v3` promotion — **not a new dependency**.

Recommended integration, following the completions precedent exactly:
- A new **hidden** cobra command, e.g. `engram man <output-dir>` (`cmd.Hidden = true`), whose `RunE` calls `doc.GenManTree(rootCmd, &doc.GenManHeader{...}, outputDir)`. Because it is `Hidden`, `cmdwalk.go:23`'s existing skip predicate excludes it from `buildCatalog`'s walk **with zero changes to `cmdwalk.go` or `internal/surfaces/toolclass.go`** — no golden regen, no classification row required, mirroring exactly how `completion` needed none.
- Extend `.goreleaser.yaml`'s `hooks.post.install` with a fourth step: `system_command binary, args: ["man", tmpdir]` then copy each generated roff file to `#{HOMEBREW_PREFIX}/share/man/man1/`, with a matching `hooks.post.uninstall` `rm_f` sweep — the SAME hand-written, `must_succeed: true` pattern already used for the version gate and completions (`01-CONTEXT.md` D-09/D-10), never Homebrew's `generate_completions_from_executable`-style rescuing helper (already explicitly rejected in this codebase, and there is no cask-native `manpages:` DSL field to reach for instead).
- `cmd/engram/releaseconfig_test.go:146`'s forbidden-strings assertion (`generate_completions_from_executable`, `brews:`, `rm_rf`) is the natural place to extend with a parallel assertion once the manpage hook lands, proving the same non-rescuing pattern, rather than inventing a new test shape.

## 6. `osRun` deadline classification — #560

### The defect, precisely

`osRun` (`internal/setup/environment.go:84-104`) runs `cmd.Run()` and classifies the result in a switch that checks ONLY `runErr == nil` vs. `errors.As(runErr, &exitErr)` vs. everything else. When `ctx`'s deadline kills the subprocess, `exec.CommandContext` still typically surfaces the failure as an `*exec.ExitError` (the process was signaled/killed, `Wait` returns that shape) — so today's code takes the `errors.As` branch, reports a clean `RunResult{ExitCode: -1}` with a **nil error**, and the caller (`runSeam`/`execute`, `apply.go:125-129,213-369`) has no way to distinguish this from an ordinary nonzero exit. `Environment.Run`'s own doc comment (`environment.go:44-49`) already states the correct contract: "a non-nil error means the process never produced an exit status at all — it failed to start, or ctx's deadline expired before it exited." A deadline kill is being silently miscategorized into the WRONG side of that contract.

### Minimal fix

Add one case at the top of the existing switch, checking `ctx.Err()` before the `errors.As` branch:

```go
runErr := cmd.Run()
result := RunResult{Stdout: stdout.String(), Stderr: stderr.String()}

var exitErr *exec.ExitError
switch {
case ctx.Err() != nil:
    return RunResult{}, ctx.Err()
case runErr == nil:
    return result, nil
case errors.As(runErr, &exitErr):
    result.ExitCode = exitErr.ExitCode()
    return result, nil
default:
    return result, runErr
}
```

This is a three-line diff, zero new imports (`context` is already imported), zero interface change, and zero cross-package impact: `runSeam`/`execute` already treat "non-nil error = seam error" correctly today (`describeSeamError`, `apply.go:148-155`, already exercised for start-failure cases) — a deadline kill starts correctly classifying as `OutcomeFailed` via the EXISTING seam-error path with no changes needed anywhere outside `environment.go`. Checking `ctx.Err()` post-`Run()` (rather than racing it against the process's own exit) is the standard idiom `os/exec`'s own documentation expects for exactly this pattern; no additional synchronization is needed. This is the smallest, most independently shippable, and lowest-risk item in the whole milestone.

## 7. New vs. modified components

| Component | New / Modified | Why |
|---|---|---|
| `internal/setup/environment.go` (`osRun`) | **Modified** | #560 fix — add `ctx.Err()` precedence check (§6) |
| `internal/setup/plan.go` (`SkillFormat` const block) | **Modified** | Add `SkillFormatPlugin` (§1) |
| `internal/setup/plan.go` (`Plan.Probe` field) | **Modified** | Generalize from one probe to a small named set — shared prerequisite for plugin-state convergence (§1) and drift comparison (§4) |
| `internal/setup/claudecode.go` | **Modified** | Append plugin-management `Action`s; add `case "header":`; add `parseClaudeCodeRegistration` |
| `internal/setup/codex.go` | **Modified** | Append plugin-management `Action`s (pending live verification); add `case "header":` with `ErrAuthModeUnsupported` for non-`Authorization` headers; add `parseCodexRegistration` (JSON, easy case) |
| `internal/setup/opencode.go` | **Modified** | Add `case "header":`; deliberately NO observation parser (documented omission, §4) |
| `internal/setup/generic.go` | **Modified** | Extend `Headers` map population for the new header mode |
| `internal/setup/runtime.go` (`Options`) | **Modified** | Add `Headers []HeaderSpec` |
| `internal/setup/aggregate.go` | **Modified** | New `OutcomePreserved` precedence tier (§4); `SkillsOutcome` gains a `SkillFormatPlugin` arm (§1) |
| `internal/setup/exit.go` (`Classify`) | **Modified** | Must recognize `OutcomePreserved` in its exhaustive switch (§4) |
| `internal/setup/observe.go` | **New** | `ObservedRegistration` struct + `compareRegistration` (§4) |
| `internal/setup/headers.go` (or inline in `runtime.go`) | **New** | `HeaderSpec` type + validation helper (§3) |
| `cmd/engram/setup.go` | **Modified** | New `--header` flag + `--auth header` validation branch; `setupSkillsTarget` gains `SkillFormatPlugin` case; `setupRuntimeRow` gains `Plugin`-facet field(s); `setupApplySummary` gains a `preserved` tally bucket |
| `internal/setupgen/setupgen.go` | **Modified** | `Render()` gains a plugin-actions filter + third table; `Cases()` gains a header-mode case (§2) |
| `skill/engram/commands/engram-setup.md` | **Modified (generated)** | Regenerated via `task surfaces:gen` once `Render()` changes (§2) — never hand-edited |
| `cmd/engram/man.go` (or similar) | **New** | Hidden `engram man <dir>` command wrapping `doc.GenManTree` (§5) |
| `.goreleaser.yaml` | **Modified** | Fourth `hooks.post.install`/`hooks.post.uninstall` step for manpages (§5) |
| `cmd/engram/releaseconfig_test.go` | **Modified** | Extend the forbidden-strings assertion for the manpage hook (§5) |
| `go.mod` | **Modified** | Promote `github.com/spf13/cobra/doc` from indirect to direct (§5) — not a new dependency |

Not modified by this milestone: `internal/skills/*` (install/inventory/agentsmd logic is untouched — plugin delivery SKIPS this package for the affected runtimes rather than changing it), `internal/surfaces/toolclass.go` (no new top-level `engram` command is added — `man` is `Hidden` and needs no classification row), `cmd/engram/cmdwalk.go` (its existing skip predicate already covers a `Hidden` command).

## 8. Build order

Ordered by genuine dependency, per the question's own framing (header model before drift comparison; plugin action before setupgen regeneration):

1. **`osRun` deadline fix (#560)** — zero dependency on anything else in this milestone; smallest, independently shippable, ship first. Test: a real short-lived subprocess (e.g. `sleep`) under a tiny `context.WithTimeout`, asserting a non-nil error and zero `RunResult` — this exercises engram's OWN exec-wrapping logic, not a third-party CLI's flag surface, so it does not run afoul of rule `m45p2b4bp7`.
2. **Manpages (`engram man` hidden command + `.goreleaser.yaml` hook)** — fully independent of every other item; small; zero interaction with `Options`/`Plan`. Land any time; grouped early here as a second quick, low-risk win.
3. **Custom headers (`Options.Headers`, `HeaderSpec`, per-runtime `case "header":` arms, `--header`/`--auth header` CLI surface, codex's capability-gap handling)** — must land BEFORE drift detection (step 5), because `compareRegistration` needs the authored header vocabulary to exist before it can compare against it. Independent of plugin delivery (step 4) — the two touch overlapping files (`claudecode.go`, `codex.go`) but not overlapping logic; sequence before step 4 only to reduce merge risk in shared files, not because of a real dependency.
4. **Plugin-first delivery (`Plan.Actions` gains plugin actions in `claudecode.go`/`codex.go`; `SkillFormatPlugin`; report row fields)** — depends on live verification of `claude plugin`/`codex plugin` CLI syntax (flagged in §1) before the exact `Action.Args` can be locked; otherwise independent of step 3.
5. **`setupgen` regeneration for plugin actions** — hard, same-commit dependency on step 4 (§2): the generated prose must reflect the real `Plan.Actions` shape the moment it changes, exactly as the prior milestone's research established for `internal/surfaces` + `buildCatalog` (a command's classification and its golden regen land atomically, never across separate PRs).
6. **Drift detection, read-only half** (`ObservedRegistration`, per-runtime parsers for claude-code/codex only, `compareRegistration`, `OutcomePreserved` touching the three exhaustive sites, report-only surfacing) — depends on steps 3 and 4 being stable, since the comparison surface (headers, plugin state) must exist before it can be compared against. This is the largest and riskiest single slice; do not start it until 1–5 are merged and stable.
7. **Drift detection, reconcile half** (the optional `Reconcile` hook; write-time merging of preserved elements into the live write) — depends on step 6; recommend treating this as an explicitly separable follow-on within the milestone (or a deliberately deferred slice, per the milestone's own tolerance for carrying "hand-edit reconciliation" forward if it does not fit), never bundled into step 6's own commit.

`REQ-register-cursor` stays out of scope per `PROJECT.md`'s explicit deferral and is not sequenced here.

## Zero-new-Go-dependencies check

Every mechanism above is stdlib-plus-already-vendored: `context`/`os/exec` (already used, §6), `encoding/json` for codex's structured probe (§4), `go/doc` — actually `github.com/spf13/cobra/doc`, already an indirect module dependency per the milestone's own framing, promoted to direct (metadata-only, §5) — and cobra/pflag for the new `--header` flag and hidden `man` command (already a direct dependency). No new third-party config-format parser enters the tree at any point: plugin management is argv shelled to `claude`/`codex`'s own CLI (same pattern as `mcp add` today); headers are argv strings or a JSON map already produced by stdlib `encoding/json` (`generic.go:147-149`); drift observation for codex uses stdlib JSON decoding of codex's own `--json` probe output; opencode's probe is explicitly NOT parsed, so no format-specific dependency is needed or wanted there. **No proposal in this document requires a new Go dependency.**

## Sources

All findings above are grounded in repository reads performed in this research pass:

- `.planning/PROJECT.md` ("Current Milestone: 2026-09-13.01 Setup v2" section; prior-milestone "Carried tech debt"/"Deferred" entries)
- `.planning/research/ARCHITECTURE.md` (previous milestone's research — read first, superseded by this file per the milestone's own instruction)
- `cmd/engram/setup.go` (whole file)
- `internal/setup/plan.go`, `runtime.go`, `environment.go`, `apply.go`, `aggregate.go`, `exit.go`, `claudecode.go`, `codex.go`, `opencode.go`, `generic.go`
- `internal/skills/install.go`, `agentsmd.go`, `environment.go`, `inventory.go`
- `internal/setupgen/setupgen.go`
- `skill/engram/.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`
- `cmd/engram/cmdwalk.go` (`commandWalkSkip` predicate proving `completion` is already live and deliberately unclassified)
- `.goreleaser.yaml` (existing `homebrew_casks.hooks.post.install`/`post.uninstall` completions wiring)
- `.planning/milestones/2026-08-23.01-phases/01-version-homebrew-distribution/01-CONTEXT.md`, `01-RESEARCH.md`, `01-SUMMARY.md` (D-09/D-10: completions already shipped via `system_command`, never `generate_completions_from_executable`)
- `cmd/engram/releaseconfig_test.go` (forbidden-strings assertion pattern)
- `.planning/BACKLOG.md` (checked; items 999.5/999.6 detail lives in `PROJECT.md`'s milestone section, not as separately numbered backlog prose)

---
*Architecture research for: engram `setup` v2 — plugin delivery, custom headers, drift detection, completions*
*Researched: 2026-09-13*
