# Feature Research: `engram setup` v2 (Plugin Delivery, Custom Auth, Drift, Completions)

**Domain:** CLI installer extending an already-shipped multi-runtime MCP bootstrap
**Milestone:** 2026-09-13.01 — Setup v2
**Researched:** 2026-09-13
**Confidence:** MEDIUM-HIGH overall — HIGH for Claude Code plugin CLI, header syntax, and
Codex MCP registration flags (primary vendor docs, cross-checked against the shipped
`internal/setup/*.go` source in this repo); MEDIUM for Codex's plugin CLI (young feature,
best source is an in-flight PR, not a stable docs page); LOW/version-dependent for whether
`codex mcp add` carries a generic custom-header flag (flagged explicitly below — the
milestone's own PROJECT.md context already treats this as settled: `--bearer-token-env-var`
only, no generic header flag; live-verify against the installed `codex` binary before
committing an implementation, matching this repo's own precedent of live-verifying CLI
surfaces before coding against them).

This file supersedes the FEATURES.md from milestone 2026-08-23.01 (Distribution & Agent
Bootstrap), which is now historical baseline — its table-stakes items (detect-by-PATH,
preview-by-default, merge-never-clobber, non-interactive flags, native skill formats) are
now **shipped** and are treated here as load-bearing prerequisites, not open questions.

## Correction Surfaced By This Research (read before phase planning)

PROJECT.md's milestone context describes the completions/manpages item as "cobra's
auto-registered `completion` plus `cobra/doc`... the cask's `generate_completions_from_executable`
hook already expects a completion verb." Reading the actual shipped
`.goreleaser.yaml`/`releaseconfig_test.go` shows this is imprecise in one respect worth
correcting before scoping: **shell completions are already fully shipped**, not partially
scaffolded. The cask's `post_install` hook already calls `engram completion <shell>`
(cobra's auto-registered command, live-exercised — a broken binary fails cask install
rather than installing with a swallowed warning) and writes bash/zsh/fish completion
files; `post_uninstall` already removes them; `releaseconfig_test.go` already pins the
ordering (version-check before completion-generation) and **forbids** re-introducing the
declarative `generate_completions_from_executable` Cask DSL field by literal occurrence
count. **Man pages are the only genuinely new surface in this category** — `cobra/doc` is
an indirect dependency already in `go.sum` but nothing in the tree calls it
(`GenManTree`, `GenMarkdownTree`, etc. — zero occurrences). Scope and complexity estimates
below reflect this: "shell completions" carries near-zero remaining work; "man pages" is
the real item.

---

## Category 1 — Plugin-First Delivery

### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---|---|---|---|
| Detect plugin-CLI *capability*, not just runtime presence | A runtime binary can be on `PATH` while its `plugin` subcommand is absent (pre-plugin-era version) or non-functional — the exact "capability vs. binary" gap this repo already treats as a first-class failure mode for `--auth` support (`opencode` + `oauth-client` → a `failed` row with a reason, not a silent skip) | LOW-MEDIUM | Probe `claude plugin list --json` / `codex plugin list` and treat a nonzero exit or unrecognized-subcommand error as "plugin delivery unsupported for this runtime, fall back to native skill copy" — never as a hard failure of the whole runtime row |
| Never install a plugin when one already satisfies the requirement | Anthropic's own docs (code.claude.com/docs/en/plugins-reference, /discover-plugins) warn plugins "can execute arbitrary code on your machine" — re-running an installer that reinstalls/reclones an already-correct plugin on every invocation is both wasteful and a trust problem, matching this repo's own idempotent-reapply convention | LOW | `claude plugin list --json` returns installed plugins with enough identity (`name@marketplace`) to test presence before calling `install` |
| Skip the plain skills copy on the plugin-delivery path, entirely | This is the milestone's literal motivation (backlog 999.6/999.5): the maintainer's real-machine failure was `--apply` writing a duplicate `curating-memory` skill next to the plugin's `engram:curating-memory`, and replacing Codex's marketplace symlinks with static files | LOW | The two delivery modes (plugin vs. native skills copy) must be mutually exclusive per runtime per run — extend the existing `setup.SkillTarget`/`skills.Target` seam with a `FormatPluginManaged` (or equivalent no-op-for-skills) value analogous to the already-shipped `FormatNone` used by `generic`, rather than teaching the skills package about plugins directly |
| Add engram's own marketplace/plugin source, never a foreign one, without asking | The milestone's plugin is engram's own (`skill/engram/.claude-plugin/plugin.json`/`marketplace.json`, already shipped) — registering *that* source is in scope; auto-adding or auto-trusting some *other* discovered marketplace is not, and Anthropic's docs draw exactly this trust line ("Only install plugins and add marketplaces from sources you trust") | LOW | `claude plugin marketplace add <engram's own source>` is deterministic and known at build time — no user choice needed, unlike a generic marketplace picker |
| Report plugin delivery as its own facet, mirroring the shipped Registration/Skills split | `setupRuntimeRow` already carries two independently-reported facets (`Registration`, `Skills`) aggregated via `setup.AggregateOutcome` — a `wrote` registration next to a `failed` plugin-install must remain visible, not collapsed | LOW-MEDIUM | Reuse the existing two-facet aggregation shape; a third facet ("Delivery": `plugin` \| `skills-copy`) is cleaner than overloading the existing `Skills` field with two different meanings |
| Claude Code: `--json` on every plugin subcommand for scriptable, parseable install/update | `claude plugin install/uninstall/update/list/enable/disable --json` (v2.1.268+) all emit a structured envelope (`{"command","outcome":"ok"\|"failed","message","pluginId","scope","failureCode"}`) on their **last line**, with marketplace-refresh chatter printed *ahead* of it — this is exactly the "parse the final JSON line only" contract already familiar from `engram setup --output json`'s own convention | LOW | Confirmed HIGH-confidence primary docs (code.claude.com/docs/en/plugins-reference) |
| Codex: use `codex plugin marketplace add`/`plugin add`/`plugin list`/`plugin remove` | Confirmed to exist (openai/codex PR #21396, primary source: the actual CLI arg definitions) — `plugin add <PLUGIN[@MARKETPLACE]>`, `plugin marketplace add <SOURCE> [--ref REF] [--sparse PATH]`, `plugin marketplace upgrade [NAME]`, `plugin marketplace remove <NAME>`, `plugin remove <PLUGIN[@MARKETPLACE]>` | MEDIUM | **No `--json` flag confirmed on any Codex plugin subcommand** in the PR under review — treat Codex's plugin delivery as text-output-only until verified otherwise against the installed binary; this materially lowers how much of the report can be machine-checked for Codex vs. Claude Code |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---|---|---|---|
| Auto-detect "plugin already installed and current" vs. "installed but outdated" vs. "absent" as three distinct outcomes | Neither vendor's plugin CLI exposes a clean, documented "up to date" vs. "updated" distinction in its own JSON envelope (Claude's `update` command's own doc only states "updates to latest version... fails if plugin not found" — no `no-op` outcome documented; Codex has no per-plugin `update` at all, only a marketplace-wide `plugin marketplace upgrade` that refreshes snapshots) — engram doing this comparison itself (installed version from `plugin list --json` vs. the version pinned in its own `plugin.json`/marketplace entry) is genuinely more precise than what either vendor CLI reports on its own | MEDIUM | This is the plugin-delivery analogue of "already-correct becomes a real comparison" (Category 3) — the same comparison discipline applies to both features and should probably share code |
| One `setup --apply` call handles plugin-vs-skills-copy transparently per runtime | No surveyed prior-art multi-runtime installer (getmcp, mx setup, mcp-config) has to choose between two entirely different *delivery mechanisms* for the same payload (skills) depending on what's already present on the machine — this is a genuinely novel shape born from Claude Code and Codex acquiring first-party plugin managers only in 2026 | MEDIUM-HIGH | The differentiator is honest reconciliation of "what's already there," not offering more delivery mechanisms — see Anti-Features below |

### Anti-Features

| Anti-Feature | Why It Seems Appealing | Why Problematic | Do Instead |
|---|---|---|---|
| Installing/updating the engram plugin without `--apply` | "Zero friction" | Plugins execute arbitrary code (Anthropic's own words) — this is *more* invasive than the already-shipped skills-copy path, so it must sit behind at least the same gate, arguably a stricter one | Keep behind `--apply`; preview must show the exact `claude plugin install .../codex plugin add ...` invocation, not just a summary sentence |
| Silently reinstalling a plugin the maintainer's machine already manages via marketplace auto-update | "Consistency — always converge to the pinned version" | This is precisely the reported real-world bug (backlog 999.6): Claude's own marketplace auto-update may already be tracking `latest`, and a forced reinstall from `engram setup` can fight that mechanism or downgrade/pin unexpectedly | Detect present-and-tracked-by-marketplace as `already-correct`; only act when genuinely absent or when the installed source doesn't match engram's own marketplace entry |
| Writing plain skill files into `~/.claude/skills/` (or Codex's `~/.agents/skills/`) *in addition to* the plugin, "just in case" | "Belt and suspenders" | This is the exact defect this milestone exists to fix — duplicate `curating-memory` next to `engram:curating-memory`, replacing Codex's marketplace symlinks with static files pinned to a stale embedded version | Delivery modes are exclusive per runtime per run: plugin-capable → plugin only; not plugin-capable → native skills copy only (unchanged shipped behavior) |
| Silently falling back to the plain skills copy when the plugin CLI errors, with no reported reason | "Best effort, don't block on plugin flakiness" | Contradicts the shipped principle that an unsupported/failed facet is always a named `failed` row, never a silent substitution the operator has to infer from a diff | Report a `failed` delivery facet naming the plugin-CLI error; require an explicit re-run or flag to fall back to skills-copy, mirroring how an unsupported `(runtime, auth)` pair already fails loudly rather than silently degrading |

---

## Category 2 — Custom Auth Headers / Auth Keys

### Grounding from the shipped code (`internal/setup/*.go`, read directly — not inferred)

The milestone's framing ("`--auth` accepts only `oauth\|oauth-client\|bearer\|none`, and
`bearer` is hard-wired to `Authorization: Bearer ${ENGRAM_TOKEN}`") is accurate for
*today's shipped behavior*, but the per-runtime CLIs it shells out to are **already more
expressive than engram currently uses**:

| Runtime | Shipped bearer invocation today | Underlying CLI capability (already present, unused for anything but `Authorization`) |
|---|---|---|
| `claude-code` | `claude mcp add ... --header "Authorization: Bearer ${ENGRAM_TOKEN}"` | `--header` is a repeatable, arbitrary `"Name: value"` flag — HIGH confidence, confirmed by both Anthropic's docs and community write-ups (`claude mcp add --header "x-litellm-api-key: Bearer sk-..."` is a documented LiteLLM pattern) |
| `codex` | `codex mcp add ... --bearer-token-env-var ENGRAM_TOKEN` | Confirmed narrow — this flag only ever produces an `Authorization: Bearer <env>` shape. The broader TOML schema Codex actually reads (`http_headers` for literal values, `env_http_headers` for `{"HeaderName": "ENV_VAR_NAME"}` pairs) is NOT confirmed reachable through `codex mcp add` itself — openai/codex#5180 ("Add support for custom headers for streamable HTTP MCP servers") shows this was an open feature request, and secondary sources explicitly hedge with "check `codex mcp add --help` on your build." **Live-verify before implementing; PROJECT.md's own framing treats the CLI as bearer-only, which this research corroborates as the safer assumption.** |
| `opencode` | `codex`-analogous: `--header "Authorization=Bearer {env:ENGRAM_TOKEN}"` (KEY=VALUE, not colon-space — already a fixed, live-verified bug per the shipped code comment) | Same flag, any header name: opencode's own docs independently confirm arbitrary `headers: {"X-Name": "{env:VAR}"}` in `opencode.json`, and disabling `oauth: false` for API-key-style auth |
| `generic` | `Headers map[string]string` populated only for `bearer`, key hardcoded to `"Authorization"` | Already a map — trivially generalizable |

**Implication:** for `claude-code`, `opencode`, and `generic`, expressing an arbitrary
header name (`x-litellm-api-key`) plus an arbitrary env-var-reference value is a
**parameter-generalization** of an already-shipped code path, not new CLI capability
research. `codex` is the one runtime where the underlying CLI's expressiveness is
genuinely uncertain and needs a live check.

### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---|---|---|---|
| Accept a header **name** in addition to a header **value reference** | This is the entire feature — LiteLLM's `x-litellm-api-key: Bearer <key>` cannot be expressed today because the header name is hardcoded to `Authorization` in three of four runtime writers | LOW (claude-code, opencode, generic); MEDIUM (codex, pending live verification) | A new `--header-name`/`--auth-header` flag (or an `--auth gateway`-shaped extension) replacing the implicit `"Authorization"` constant in `claudecode.go`, `opencode.go`, `generic.go` |
| Value stays an env-var **reference**, never a literal, on every runtime including the new shape | Standing constraint carried explicitly forward in PROJECT.md; a secret on argv or in a written config file is a stronger regression than the header-name limitation this milestone exists to fix | LOW | No behavior change needed here — extend the existing `bearerProvenance`/`${ENGRAM_TOKEN}`-reference pattern to the generalized header name, don't introduce a second secret-handling path |
| Never echo the resolved secret value anywhere in preview, apply output, or logs | Universal across every prior-art example surveyed (engram's own docs already state this for the existing bearer path: "the token never appears on any command line or in any config engram writes") | LOW | Preview must render the **reference form** (`${ENGRAM_TOKEN}` or `{env:ENGRAM_TOKEN}`), never a resolved value, exactly as today |
| `--auth bearer` keeps working unmodified when no custom header is requested | Backward compatibility — this is an additive capability, not a breaking change to the four accepted `--auth` values | LOW | The new header-name flag should be optional, defaulting to today's `Authorization` behavior when omitted |
| Reject the combination the runtime cannot express, the same way `oauth-client` is already rejected for unsupported runtimes | `opencode` + `oauth-client` already fails as an unsupported `(runtime, auth)` pair with a named reason — a custom header name Codex's CLI can't accept (pending verification) must fail the same way, never silently downgrade to `Authorization` | LOW-MEDIUM | Reuses the existing `ErrAuthModeUnsupported`-shaped mechanism; extend it to also gate on header-name support once Codex's actual capability is confirmed |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---|---|---|---|
| First-party support for the specific LiteLLM/gateway header shape, verified against the actual product | Generic MCP installers (`getmcp`, `mcp-config`) treat headers as an opaque pass-through the *user* must already know how to fill in; engram naming the exact LiteLLM convention (`x-litellm-api-key: Bearer <key>`) in its own `--help`/docs, and reproducing it correctly through `--apply`, is a concrete fix for a reported real incident (engram gotcha `ryr82bf2s2`), not a speculative feature | LOW-MEDIUM | The value is in *correctness of reproduction* (Category 3), not in inventing new syntax |

### Anti-Features

| Anti-Feature | Why It Seems Appealing | Why Problematic | Do Instead |
|---|---|---|---|
| A generic "arbitrary key=value config passthrough" flag that lets a caller inject anything into the written config | "Maximum flexibility, solves every gateway shape at once" | Reopens exactly the parsed-third-party-config-format risk this repo has structurally avoided since v0.16.0 ("every runtime writer is a shell-out to the runtime's own CLI... no third-party config format" — rule `m45p2b4bp7`-adjacent standing constraint); an unbounded passthrough also makes secret-redaction and drift-comparison (Category 3) intractable, since the tool no longer knows what shape it wrote | Scope this milestone to exactly one new degree of freedom — the header **name** — keeping the value strictly an env-var reference and the header count bounded to what each runtime's own CLI accepts |
| Accepting a literal header value on the command line "for convenience, just this once, for non-secret headers" | Some headers genuinely aren't secrets (e.g., `X-Client-Version: 3`) | Blurs the one bright line this feature depends on for safety; a caller who *thinks* a header is non-secret is exactly the failure mode the whole env-var-reference design defends against | Keep the env-var-reference requirement universal, even for headers a caller believes are safe to inline |

---

## Category 3 — Drift Detection + Reconcile Hand-Edits

### Prior art convergence (Terraform `plan`/`refresh`, Ansible `--check --diff`, pre-commit's "migration mode")

Every mature convergence tool in this space separates exactly three states, not two:

1. **Matches what I'd write** → no action (`already-correct`, already the shipped outcome name).
2. **Differs, and I can express the difference** → previewed as a concrete replacement, applied only under `--apply` (this is what "would-write"/"wrote" already mean in the shipped taxonomy — the gap is that today's "differs" detection is a coarse read-probe, not a structural comparison of URL/auth-shape/headers).
3. **Differs, and I cannot express what's there** → **must never be silently classified into (2).** Terraform's own drift-detection literature is explicit that an out-of-band change the tool doesn't understand should be surfaced, not overwritten; pre-commit's "migration mode" is the sharpest analogue — an existing hook pre-commit didn't install is *moved aside and preserved*, never deleted, with the preservation stated in the tool's own output.

This milestone's own framing ("a registration `setup` cannot reproduce is reported as
preserved, never as drift to replace") is state (3) above, and is the harder half of the
three — Terraform, Ansible, and pre-commit all treat it as the case requiring the most
deliberate design, not an edge case to bolt on.

### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---|---|---|---|
| Compare the **full** existing registration (URL, auth shape, header set), not just presence | The milestone's own stated gap: `already-correct` today is "a read-probe heuristic," not a real comparison — `codex mcp get engram --json` and `claude mcp get engram --json` (implied by the existing `Registered` field/probe pattern) already return enough structure to diff against the `Plan`'s own `Action` | MEDIUM-HIGH | Requires parsing each runtime's own probe-command JSON output into the same shape the `Plan` already authors (URL, auth mode, header map) — a small, bounded parser per runtime, not a general config-format parser (keeps the standing zero-new-dependency, no-third-party-format-parsing constraint intact, since this parses the runtime's *own CLI's own JSON output*, not its underlying config file) |
| Three-way classification: identical / reproducible-diff / non-reproducible | This is the entire feature — collapsing (2) and (3) is the exact bug this milestone exists to prevent (the 2026-09-10 overwrite, gotcha `ryr82bf2s2`) | MEDIUM | Extend `setup.Outcome` with a value distinct from `wrote`/`already-correct`/`failed` for case (3) — e.g. a `preserved` outcome — analogous to how `not-present` is already a fourth non-failure state alongside the write outcomes |
| `preserved` is reported, never silently absorbed into `already-correct` | A silent no-op is indistinguishable from a bug to the operator (this exact principle is already stated in the prior milestone's own research and echoed by `mise`'s and `pre-commit`'s explicit skip-logging) | LOW | `preserved` must appear as its own outcome/row detail, with a reason naming *what* couldn't be reproduced (e.g., "existing header set includes `X-Custom-Signing` which engram does not author") |
| Preview shows the comparison result before `--apply`, exactly like today's registration/skills rows | Standing convention (preview-by-default, `--apply` mutates) — drift comparison is a **read-only** enrichment of the existing preview, not a new command | LOW | The comparison itself never mutates; it only changes what a `would-write` row's `Reason`/`Notes` field says |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---|---|---|---|
| Naming *which specific facet* differs (URL vs. auth mode vs. header name vs. header value-reference) rather than a bare "differs" | Generic MCP installers surveyed previously (getmcp, mx setup) report "will update the entry" without stating which field changed; naming the facet is what lets an operator trust a `wrote` outcome touched only what they expected | MEDIUM | Directly reuses the parsed-comparison structure from the table-stakes row above — this is presentation of the same data, not new mechanism |
| `already-correct` becomes provably a real comparison (testable, not "probably fine") | The milestone explicitly calls this out as a required outcome, and it is exactly the kind of property this repo already tests structurally (e.g., `TestOperatorViewFixturesHaveNoUnsanitizedNesting`, `TestDestructiveCommandsRouteThroughGate`) rather than by convention | MEDIUM | A dedicated comparison function with unit coverage for "identical," "differs-reproducible," and "differs-non-reproducible" cases per runtime is the natural shape |

### Anti-Features

| Anti-Feature | Why It Seems Appealing | Why Problematic | Do Instead |
|---|---|---|---| 
| Auto-migrating/rewriting a hand-edited registration to engram's canonical shape without `--apply`, or without a named reason in preview | "Just fix it for them" | This is precisely the cardinal sin identified across every surveyed tool (getmcp: "never overwrites"; mx setup: "other servers are untouched") and the milestone's own stated goal is the opposite — never replace what it cannot reproduce | Preserve; report the non-reproducible facet by name; let the operator decide, matching pre-commit's "migration mode" transparency |
| Treating "I can't parse the existing registration's headers" as a hard failure of the whole runtime row | "Fail loud, fail safe" | An unparseable extra header is not an error — it is exactly the reproducibility gap this feature is designed to name and preserve, not reject | Classify as `preserved` (a non-failure outcome), never `failed`, unless the registration is entirely absent/corrupt in a way that blocks read at all |
| Silently widening "already-correct" to include "close enough" (e.g., ignoring header ordering, case, or an extra runtime-injected header) without stating the tolerance | "Reduce false-positive drift noise" | An undocumented tolerance is itself a hidden behavior a future contributor or auditor cannot verify — exactly the class of implicit convention this repo's Nyquist/surfaces-conformance discipline exists to prevent | If a tolerance is needed (e.g., a runtime injects its own default header engram never authored), name it explicitly in code and in the reported reason, not as silent equality-relaxation |

---

## Category 4 — Shell Completions + Man Pages

### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---|---|---|---|
| Shell completions for bash/zsh/fish | **Already shipped** — cobra's auto-registered `completion` command plus the cask's hand-rolled `post_install`/`post_uninstall` hooks (`.goreleaser.yaml`) that exercise the real binary and fail the install if it can't produce completions | **DONE — zero remaining work** | Do not re-scope this as new work; verify it stays this way (`releaseconfig_test.go` already pins the ordering and forbids the declarative Homebrew DSL field by occurrence count) |
| Man pages, generated from the same cobra command tree | `cobra/doc`'s `GenManTree`/`GenMarkdownTree` walk the live command tree the same way the shipped `completion` command and the existing golden-help tests already do — zero new Go dependency, since `cobra/doc` is already indirect in `go.sum` | LOW-MEDIUM | Mirror the completions precedent exactly: a hidden/internal generation path invoked by the binary itself (not a build-time-only script divorced from the actual released binary), so a broken command tree fails the same way a broken completions generator does today |
| Man pages installed and removed by the cask, symmetric with completions | The completions hook already demonstrates the pattern (write on `post_install`, `rm_f` the exact paths on `post_uninstall`, never a recursive directory removal) | LOW | `#{HOMEBREW_PREFIX}/share/man/man1/engram.1` (and per-subcommand pages if `GenManTree` produces one per command, matching `GenManTree`'s documented per-command-and-descendants output) — note cobra's own docs flag a naming caveat for hyphenated command names ("If you have a `sub`/`sub-third` split it is undefined which file wins") that engram's flat-ish command tree likely avoids but should be checked |
| CI ordering/golden coverage matching the completions precedent | The existing `releaseconfig_test.go` already treats "version-check before completion-generation" and "the declarative DSL field is absent" as tested properties, not conventions | LOW-MEDIUM | Add the equivalent assertions for the man-page generation step once it exists, rather than leaving it as an untested cask-script addition |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---|---|---|---|
| Man pages generated from the live command tree rather than hand-maintained prose | Zero drift between `--help` output and the shipped man page — the same "correct by construction, not by convention" discipline this repo already applies to its CLI catalog goldens and `setupLongDescription` (which derives its runtime list from `setup.Names()` rather than a hand-typed string) | LOW | Not a differentiator over other CLIs generally (most mature Cobra-based tools already do this — kubectl, helm, gh), but is a differentiator over engram's own prior state (zero man pages today) |

### Anti-Features

| Anti-Feature | Why It Seems Appealing | Why Problematic | Do Instead |
|---|---|---|---|
| Hand-writing static man page(s) once and committing them | "Ship something now" | Guaranteed to drift the moment a flag or subcommand changes — this repo has repeatedly treated exactly this class of drift (help text, catalog goldens, `setupLongDescription`) as a defect worth a dedicated gate, not a documentation nit | Generate from the live `cobra.Command` tree via `cobra/doc`, exercised through the real binary at release/install time, matching the completions precedent |
| Re-introducing Homebrew's declarative `generate_completions_from_executable` (or an equivalent man-page helper) for either surface | "Less code to maintain" | This repo already rejected that path explicitly for completions, with a test enforcing the rejection by occurrence count, because the helper swallows a failing binary as a warning instead of failing the install — the same swallowed-failure risk applies to any equivalent man-page helper | Hand-roll the man-page install/uninstall hooks the same way completions are hand-rolled, exercising the real installed binary and letting a failure raise |

---

## Feature Dependencies

```
Plugin-first delivery
    └──requires──> capability detection distinct from binary-on-PATH detection
                    (claude plugin list --json / codex plugin list, both read-only)
    └──requires──> a third reportable facet (Delivery) alongside the shipped
                    Registration/Skills facets, OR a repurposed Skills facet value
                    that means "plugin-managed, no filesystem write performed"
    └──conflicts with──> writing the plain native skills copy for the same
                    runtime in the same run (must be exclusive, per the milestone's
                    own stated motivation)
    └──lower-confidence-on──> Codex's plugin CLI scriptability (no confirmed --json;
                    best source is an in-flight PR, not a stable docs page) — verify
                    live against the installed `codex` binary before locking behavior

Custom auth headers
    └──requires──> generalizing the hardcoded "Authorization" header-name constant
                    in claudecode.go / opencode.go / generic.go (LOW complexity —
                    three already-shipped writers, same env-var-reference pattern)
    └──requires──> live-verifying whether `codex mcp add` accepts a header-name flag
                    at all, or whether Codex needs a scoped TOML edit limited to the
                    env_http_headers key of the ALREADY-EXISTING [mcp_servers.engram]
                    table (created by `codex mcp add` itself) — this is a much
                    narrower risk than the prior milestone's full-TOML-writer question,
                    since MCP registration for Codex is already confirmed to be a
                    shell-out, not a file-write
    └──shares design with──> drift detection's structural comparison (both need the
                    same "parsed header name/value-reference shape" as their unit of
                    comparison — build once, use in both features)

Drift detection + reconcile hand-edits
    └──requires──> parsing each runtime's own read-probe JSON output into the same
                    shape the Plan already authors (bounded, per-runtime, NOT a
                    general third-party config-file parser — keeps the standing
                    zero-new-dependency / shell-out-only constraint intact)
    └──requires──> a new non-failure Outcome value ("preserved") distinct from
                    already-correct/wrote/failed/not-present
    └──requires──> custom auth headers landing first (or concurrently) — you cannot
                    correctly classify "differs because of an unreproducible header"
                    until the header-name generalization exists to even attempt
                    reproduction

Shell completions + man pages
    └──requires──> NOTHING new for shell completions (already shipped — do not
                    re-plan this half)
    └──requires──> cobra/doc (already an indirect dependency, zero new Go deps) for
                    man-page generation, invoked through the real binary the same
                    way `engram completion <shell>` already is
    └──independent-of──> the other three features — no shared code, can land in
                    any order relative to them

#560 osRun deadline classification
    └──independent-of──> all four features above — a correctness fix in
                    internal/setup/environment.go's subprocess-timeout handling,
                    isolated from the delivery/auth/drift/docs work
```

## MVP Recommendation

Prioritize, in dependency order:

1. **Custom auth headers (Category 2)** — smallest, most mechanical change for three of
   four runtimes (parameter generalization of an already-shipped code path); directly
   fixes a reported real-world breakage (gotcha `ryr82bf2s2`); and its parsed
   header-shape is a prerequisite building block for drift detection.
2. **Drift detection + reconcile (Category 3)** — depends on (1) for header comparison to
   be meaningful; delivers the milestone's headline promise ("never replaces a
   registration it did not write").
3. **Plugin-first delivery (Category 1)** — independent of (1)/(2) but the highest
   complexity and the one item with a genuine external-capability confidence gap
   (Codex's plugin CLI maturity); scope Claude Code's plugin path first (HIGH
   confidence, fully documented, `--json` everywhere) and treat Codex's plugin path as
   the item most likely to need a live-verification spike before implementation,
   mirroring how the prior milestone treated opencode's MCP schema uncertainty.
4. **Man pages (Category 4)** — fully independent, low complexity, no coupling to the
   other three; safe to parallelize with any of the above. Shell completions require no
   action.

Defer/verify-first:
- Whether `codex mcp add` (or `codex plugin add`) exposes any scriptable JSON output —
  treat Codex's report fidelity as text-only until confirmed otherwise.
- Whether Codex's custom-header support requires a scoped TOML edit (narrow: one key,
  one already-existing table) rather than a pure CLI flag — a 10-minute
  `codex mcp add --help` check against the installed binary resolves this before
  committing an implementation, exactly as this repo's own `03-RESEARCH.md` precedent
  did for the original bearer/URL/header syntax across all three native runtimes.

## Sources

**Primary vendor docs (HIGH confidence):**
- code.claude.com/docs/en/plugins-reference, /discover-plugins — full `claude plugin`
  CLI syntax (install/uninstall/update/list/enable/disable, `--json` v2.1.268+,
  `--scope`, version pinning via `plugin.json`/marketplace-entry `version`,
  marketplace add/list/update/remove, refresh-before-lookup semantics, security
  warning language) — fetched 2026-09-13
- litellm.ai/docs/mcp, /docs/mcp_oauth, /docs/auth_overview,
  github.com/BerriAI/litellm PR #12460 — `x-litellm-api-key` header shape, the exact
  gateway pattern this milestone must reproduce, and `claude mcp add --header` usage
  against it
- developers.openai.com/codex/mcp (via redirect to learn.chatgpt.com/docs/plugins for
  the plugins page) — Codex TOML `http_headers`/`env_http_headers`/`http_headers_helper`
  schema, `~/.codex/config.toml` vs. `.codex/config.toml` (trusted-projects-only) scoping
- opencode.ai/docs/mcp-servers/, opencode.ai/docs/config/ — `headers`/`{env:VAR}`
  interpolation, `oauth: false` for API-key auth, KEY=VALUE `--header` shape
  (independently corroborates the shipped `internal/setup/opencode.go` comment)
- Homebrew Cask Cookbook (docs.brew.sh/Cask-Cookbook),
  github.com/Homebrew/brew PR #21781/#21293 — `generate_completions_from_executable`
  DSL and its rejected-by-this-repo swallowed-failure behavior
- pkg.go.dev/github.com/spf13/cobra/doc — `GenManTree`/`GenMarkdownTree` API and the
  hyphenated-command-name caveat

**Primary source code (HIGH confidence, this repository):**
- `internal/setup/claudecode.go`, `codex.go`, `opencode.go`, `generic.go` — shipped
  bearer-header invocations per runtime, read directly rather than inferred
- `cmd/engram/setup.go` — outcome vocabulary, exit taxonomy, Registration/Skills facet
  aggregation, `registerDestructive` preview/apply gate
- `.goreleaser.yaml`, `cmd/engram/releaseconfig_test.go` — the actual shipped
  completions mechanism (hand-rolled, not the declarative Homebrew DSL), which
  corrects PROJECT.md's own framing of this milestone item
- `docs-site/src/content/docs/guides/agent-setup.md` — shipped outcome-vocabulary
  table (`not-present`/`would-write`/`already-correct`/`wrote`/`failed`) this research
  extends with a proposed `preserved` value

**Primary source, in-flight (MEDIUM confidence — not yet a stable release):**
- github.com/openai/codex/pull/21396 — `codex plugin`/`codex plugin marketplace`
  subcommand definitions (add/list/remove/upgrade); no `--json` flag found
- github.com/openai/codex/issues/5180 — open request for custom-header support on
  streamable-HTTP MCP servers via the CLI, evidence that CLI-level header support is
  newer/less certain than the TOML schema itself

**Third-party/community (MEDIUM confidence, corroborating only):**
- Prior-art convergence on the three-way drift classification (identical /
  reproducible-diff / non-reproducible): spacelift.io and scalr.com Terraform
  drift-detection guides, docs.ansible.com check-mode/diff-mode docs,
  pre-commit.com + `pre_commit/commands/install_uninstall.py`'s "migration mode"
  (all carried forward from the prior milestone's FEATURES.md, re-applied here to the
  reconcile-hand-edits requirement specifically)
- codex.danielvaughan.com (2026-04 to 2026-06 posts) — corroborating detail on Codex
  plugin cache paths (`~/.codex/plugins/cache/...`) and marketplace commands; treated
  as MEDIUM since it is a single unofficial blog, cross-checked against the PR source
  above rather than trusted alone

---
*Feature research for: engram `engram setup` v2 — plugin delivery, custom auth headers,
drift detection, completions/manpages*
*Researched: 2026-09-13*
