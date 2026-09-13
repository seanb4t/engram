# Pitfalls Research — Setup v2 (2026-09-13.01)

**Domain:** Extending an already-shipped, idempotent multi-runtime installer
(`engram setup`, v0.16.x) with plugin-first delivery, custom auth headers, drift
detection/reconcile, and cobra completions/manpages — engram
(/Volumes/Code/github.com/seanb4t/engram).
**Researched:** 2026-09-13
**Confidence:** HIGH for anything cited against this repo's own shipped code
(`internal/setup/*.go`, `internal/skills/*.go`, `.goreleaser.yaml`,
`cmd/engram/releaseconfig_test.go`) and against live, read-only `--help`
output from `claude`/`codex` installed on this machine (claude 2.1.270,
codex-cli 0.154.0) captured this session. MEDIUM for opencode CLI claims,
which are carried forward from `opencode.go`'s own code comments (live-verified
against opencode 1.18.20 in the PRIOR milestone) rather than re-verified this
session — `opencode --version`/`opencode mcp list --help` were invoked this
session and both were killed (`Killed: 9`) by the sandbox before producing
output; no mutating opencode command was attempted per the STRICT quality gate.
LOW/speculative flagged inline for anything not yet decided by this milestone
(e.g., the exact plugin-consent UX, since no phase plan exists yet).

**Carried forward from the prior milestone's `PITFALLS.md`** (2026-08-23.01,
still load-bearing and NOT restated in full below — read that file's git
history if needed): Pitfall 7 (non-atomic writes / symlinked dotfiles — now
directly evidenced by `internal/skills/install.go`'s own `installAgentsMDIndex`
doc comment, which independently arrived at the same "write through, never
stage-and-rename" symlink-preservation conclusion for `AGENTS.md`), Pitfall 8
(non-uniform runtime config paths), Pitfall 10 (config-dir presence proves
nothing), Pitfall 11 (version skew), Pitfall 14 (testing against real
dotfiles — now enforced in shipped code via the `Environment`/`skills.Environment`
injectable seams in `cmd/engram/setup.go`). Pitfalls 1–6, 9, 12, 13, 15 from
that file were Homebrew-cask/release-pipeline and two-paths-equivalence
pitfalls already resolved by v0.16.0's shipped `.goreleaser.yaml` and
`internal/setupgen` — superseded, not carried forward, except where this
milestone's new features can regress them (flagged explicitly below, e.g.
Pitfall 9 here on completions/manpages regressing the cask-install-gate
acceptance test).

## Critical Pitfalls

### Pitfall 1: `--apply` still has no opt-out from registration+skills, and that gap is the root cause the 2026-09-10 incident already proved live

**What goes wrong:**
The shipped `execute()` (`internal/setup/apply.go`) and `cmd/engram/setup.go`
give `--apply` exactly one behavior per selected runtime: run the full write
sequence (remove-then-add for claude-code, a single overwrite-add for codex,
add for opencode) AND the skills facet, together, with no flag to apply one
without the other, and no flag to apply only if the runtime is
"not-yet-registered." This is not a hypothetical — it is the documented
mechanism of the incident this milestone exists to prevent (engram memory
`ryr82bf2s2`): an agent ran the plan's own documented verification command,
`engram setup --url https://engram.example.com/mcp --apply`, to build a test
fixture, and it overwrote the maintainer's real registrations in all three
runtimes because there was and is no "only register the fake fixture URL if
nothing real is already there" mode. **Drift detection alone does not close
this gap** — a drift/reconcile feature that only changes *preview*'s
diagnosis of an existing registration does nothing to stop `--apply` from
running its unconditional remove-then-add or overwrite-add sequence the
instant a caller invokes it, fixture or not. This milestone's own PROJECT.md
already carries the corrective language ("a plan that documents `--apply` as
a verification step is an attractive nuisance") but that is process guidance
for *plan authors*, not a code-level guard.

**Why it happens:**
The existing design's philosophy (D-08's byte-compare, D-11's report-not-
diagnose) is about classifying an *already-performed* write, not about
gating whether a write happens at all — "detect drift, then decide" and
"never write blind" are easy to conflate, but they are different features.
It is tempting to believe shipping drift detection automatically makes
`--apply` safer, because the review conversation and the incident both
mention "drift" — but the incident's actual failure was an unconditional
write with no comparison step *before* the write, and drift detection as
scoped (`preview compares the full existing registration ... reported as
preserved, never as drift to replace`) is explicitly a **preview-time**,
not an **apply-time**, gate.

**How to avoid:**
Decide explicitly, as a written decision this milestone records, whether
`--apply` itself consults the same full-registration comparison drift
detection performs, and refuses (or requires an additional confirmation
flag) to replace a registration it did not write and cannot fully reproduce
— not just report it in preview. The requirement text ("a registration
`setup` cannot reproduce is reported as preserved, never as drift to
replace") must be read as applying to `--apply`'s own write path, not only
to the preview report string. If the phase plan implements reconcile as
preview-only prose, that is a scope gap worth surfacing to the user before
execution, not silently accepting the same shape that caused the incident.

**Warning signs:** A test exists proving preview correctly labels an
unreproducible registration "preserved," but no test proves `--apply`
against the SAME fixture actually leaves the file/registration byte-for-byte
untouched — those are two different assertions and only the second one
would have caught the 2026-09-10 incident.

**Phase to address:** Drift-detection/reconcile phase, as its OWN explicit
requirement ("`--apply` must consult the same comparison before writing"),
verified by a fixture test that runs `--apply` (via the fake `Environment`
seam, never a real CLI) against a pre-seeded unreproducible registration and
asserts zero write actions were issued.

---

### Pitfall 2: Custom-header auth is structurally inexpressible on Codex's own `mcp add` — a naive implementation silently downgrades or drops the header

**What goes wrong:**
Live-verified this session (`codex mcp add --help`, codex-cli 0.154.0):
Codex's `mcp add` has exactly two auth-shaped flags — `--bearer-token-env-var
<ENV_VAR>` (names a variable holding a raw bearer token) and
`--oauth-client-id`/`--oauth-client-registration`/`--oauth-resource` (OAuth).
**There is no generic `--header` flag at all.** A LiteLLM-shaped registration
(`x-litellm-api-key: Bearer ${ENGRAM_TOKEN}`, an arbitrary header NAME, not
`Authorization`) cannot be expressed on Codex's CLI surface in any form —
not even as a workaround, since `--bearer-token-env-var` hardcodes the header
name to `Authorization: Bearer <token>` (per `codex.go`'s own comment: "codex
resolves the token from that named environment variable at its own
invocation time"). A naive implementation of the new "custom headers" auth
mode that reuses `--bearer-token-env-var` for a `x-litellm-api-key`-shaped
request would SILENTLY WRITE THE WRONG HEADER NAME — registering
`Authorization: Bearer ${ENGRAM_TOKEN}` when the gateway expects
`x-litellm-api-key: Bearer ${ENGRAM_TOKEN}` — reporting `wrote`/success while
producing a registration that will fail authentication the moment Codex
actually connects. This is a second, code-shaped instance of the same root
cause as the 2026-09-10 incident: a "successful" `--apply` that replaces a
working config with a broken one.

**Why it happens:**
`--bearer-token-env-var` and a generic custom-header request LOOK like the
same shape (both are "put a token from an env var into an HTTP auth
mechanism"), and Codex's own naming ("bearer token env var") invites reusing
it for anything token-shaped. The distinction — Codex's flag fixes the
header NAME to `Authorization`, a custom-header request does not — is easy
to miss without reading Codex's own `--help` line by line, which is exactly
what this session did to surface it.

**How to avoid:**
Treat "custom header name != `Authorization`" as an explicit, tested
precondition that routes Codex to the SAME preserve/report-unsupported path
the milestone already scopes for other unreproducible registrations — never
to a coerced `--bearer-token-env-var` call. Concretely: if the new auth mode
carries a header name other than `Authorization`, Codex's `Plan()` must
return an outcome equivalent to `ErrAuthModeUnsupported` (or the new
drift-detection "cannot reproduce" outcome) for Codex specifically, the same
way `openCodeRuntime.Plan` already returns `ErrAuthModeUnsupported` for
`oauth-client` — this repo already has the exact precedent to follow. Only
when the custom header IS literally `Authorization: Bearer <ref>` does
Codex's existing `--bearer-token-env-var` remain a faithful (if narrower)
expression, and that equivalence should be asserted by a test, not assumed.

**Warning signs:** A fixture test using a non-`Authorization` header name
(e.g. `x-litellm-api-key`) against Codex's `Plan()` produces an `Action`
whose `Args` contain `--bearer-token-env-var` rather than an
unsupported/preserve outcome.

**Phase to address:** Custom-headers phase, Codex sub-task — this is the
single highest-value fixture test in the whole milestone, since it directly
prevents a repeat of the incident this milestone was opened to fix, on a
runtime the milestone's own scoping text already flags as needing special
care ("must account for Codex's `--bearer-token-env-var`-only CLI").

---

### Pitfall 3: Three runtimes, three incompatible header-value syntaxes — reusing one runtime's separator for another silently breaks the registration

**What goes wrong:**
Live-verified this session and cross-checked against shipped code:
- **Claude Code** (`claude mcp add --help`, claude 2.1.270): `-H/--header
  "X-Api-Key: abc123"` — colon-space, HTTP-header-string form, one shell
  word per header, repeatable flag.
- **opencode** (`opencode.go`'s own comment, live-verified prior milestone
  against opencode 1.18.20, itself documenting a real regression this
  project already shipped a fix for): `--header KEY=VALUE` — equals sign,
  NOT colon-space. The colon-space form was "LIVE-REPRODUCED to fail
  outright ... with an immediate nonzero exit" before the fix landed.
- **Codex**: no generic header flag exists at all (Pitfall 2).

A custom-header feature built by generalizing ONE runtime's already-working
syntax into a shared helper (e.g., "format every header as `NAME: VALUE`"
because that's how bearer mode already renders it for claude-code) will
silently reproduce the exact opencode regression this project already found
and fixed once, for the NEW custom-header code path specifically — the fix
that lives in `opencode.go` today protects only the hardcoded `Authorization`
bearer case, not a new generalized header-rendering function that a
different implementer might write without re-deriving the same lesson.

**Why it happens:**
The three runtimes' CLIs are independently designed, and "pass a header" is
exactly the kind of operation that looks like it should have one shared
serialization — until the syntax difference surfaces, which for opencode
only shows up as a live nonzero exit, never a compile-time or type-level
signal.

**How to avoid:**
Keep header-value formatting authored per-runtime, in that runtime's own
file, exactly as bearer mode already does — never introduce a shared
"format a header" helper across `claudecode.go`/`codex.go`/`opencode.go`.
`AUTHORED HERE and nowhere else (D-09)` (the package doc comment in
`runtime.go`) already states this principle for the write invocation as a
whole; apply it explicitly to header-value rendering too. Write one fixture
test per runtime asserting the LITERAL separator character in the rendered
`Action.Args` (`": "` for claude-code, `"="` for opencode), not just that a
header was included, so a regression toward the wrong separator fails
immediately rather than needing a live opencode process to surface it again.

**Warning signs:** A generalized `renderHeader(name, value string) string`
function (or similar) that more than one runtime's `Plan()` calls.

**Phase to address:** Custom-headers phase — each runtime's own sub-task,
enforced by the leaf-purity/no-shared-helper discipline this package already
follows for everything else.

---

### Pitfall 4: A read-probe genuinely CAN echo a secret value — the existing "probes never see values" claim was proven for ONE shape, not for custom headers

**What goes wrong:**
`claudecode.go`'s own doc comment states a specific, narrow, already-proven
fact: for the SHIPPED bearer mode, "`claude mcp get` and the on-disk config
both echo back the literal, unexpanded `${ENGRAM_TOKEN}` text" — i.e., the
probe is safe because Claude Code stores and displays the UNEXPANDED
variable reference, never the resolved secret, for that specific header
value shape (`"Authorization: Bearer ${ENGRAM_TOKEN}"`). **This is a fact
about Claude Code's own storage/display behavior for that literal string
shape, verified once, live, at one version — it is not a structural
guarantee that generalizes to every possible header value a custom-header
feature might construct.** Two ways it can break for the new feature:
1. A custom-header value that does NOT keep the entire secret behind a
   `${VAR}`-shaped reference (e.g., a value with a literal prefix/suffix
   around the reference, or a caller-supplied raw value passed through
   unexamined) would have that literal portion echoed by `claude mcp get`
   exactly as it is stored — the existing proof covers "whole value is a
   `${VAR}` reference," not "value contains one."
2. `opencode mcp list`'s printed table is a DIFFERENT surface with no
   verified claim about its header-echo behavior at all in this repo's own
   research — `opencode.go`'s comments document `mcp list`'s TIMING and
   completeness limitations (dials every registered server, ~1.2-1.7s) but
   say nothing about whether it prints registered header VALUES or just
   header NAMES. A drift-detection feature that captures `opencode mcp
   list`'s full stdout into a comparison buffer, a JSON report field, or
   generated prose, without first establishing (live, once) whether that
   table ever prints a resolved or stored header value, risks leaking
   whatever it does print into `--output json`, a CI log, or (via
   `internal/setupgen`) the generated `/engram-setup` slash-command
   markdown — a file that ships inside the plugin and is committed to the
   repository.
3. Drift detection's OWN stated design — "preview compares the full
   existing registration (URL, auth shape, header set)" — is new work: it
   necessarily reads MORE of the runtime's registration state than the
   existing probes did (the existing probes exist only to detect
   wrote-vs-already-correct via byte-compare, D-08; they were never
   designed to extract and RENDER a structured "header set" for a human-
   or machine-readable diff). Building that structured extraction is new
   surface area for exactly the leak this pitfall describes, and it did
   not exist in the shipped v0.16.x code at all.

**Why it happens:**
The existing, narrow, already-proven safety property gets remembered as "the
probes are safe" rather than "THIS ONE probe, for THIS ONE value shape, on
THIS ONE runtime, was verified safe" — and it's natural to extend that
confidence to a materially different feature (drift's registration-state
extraction) built on the same underlying commands.

**How to avoid:**
Before drift detection ships, live-verify (once, read-only, exactly the
discipline this research session followed) what each runtime's read verb
ACTUALLY PRINTS for a header whose value is NOT a bare `${VAR}`/`{env:VAR}`
reference — this session could not do that verification itself (STRICT: no
mutating commands), so it must happen in-phase, against a throwaway
registration, before the drift feature's comparison/rendering code is
trusted. Whatever the header set comparison consumes, sanitize it through
the same "reference-only, never resolved value" contract `bearerProvenance`
and the shipped bearer modes already hold themselves to — treat any header
VALUE captured from a probe as suspect data, never render it verbatim into
`Result.Registered`, a JSON report field, or generated prose without first
confirming (per-runtime) that it can only ever be an unresolved reference,
never a literal secret. Apply `internal/setup/apply.go`'s existing
`maxCapturedBytes`/`boundCapture` discipline to this NEW field too, but
recognize that byte-bounding a leak does not un-leak it — the fix is
verifying the source never contains a resolved value in the first place,
not truncating it after the fact.

**Warning signs:** A drift-detection "header set" field in `--output json`
or a CI-committed generated markdown file that ever contains anything other
than a `${VAR}`/`{env:VAR}`-shaped reference or a header NAME with no value.

**Phase to address:** Drift-detection/reconcile phase — the live-verification
step (what does each read verb actually print for a non-bare-reference
header) is a prerequisite task, not a nice-to-have, and should block the
comparison/rendering code from landing until done.

---

### Pitfall 5: Claude Code's remove-then-add window is now ALSO the reconcile feature's collision zone — "preserve" and "converge on re-run" want opposite actions on the same registration

**What goes wrong:**
`claudecode.go`'s `claudeCodeRemoveAction` already documents, in its own
comment, the exact destructive window this milestone must not make worse:
"if the following (fatal) add action fails or is interrupted after this
action succeeds, the operator is left with NO claude-code registration where
they previously had a working one, and engram cannot restore it — it never
read the prior entry." Layering drift detection on top of the SAME runtime
creates a direct semantic collision, not just a window-timing risk:
- **Preserve** (the new reconcile requirement) means: "a registration
  `setup` cannot reproduce must never be replaced."
- **Converge on re-run** (the EXISTING idempotency contract, `docs-site
  guides/agent-setup.md`: "Repeating setup with the same inputs converges
  on the requested registration without duplicate entries") means: "running
  `--apply` again should make the machine match what was requested."
  For claude-code specifically, "converging" is IMPLEMENTED as
  remove-then-add — there is no in-place update primitive on this CLI (the
  same `claudeCodeRemoveAction` comment: "`claude mcp add` has no
  --force/--overwrite flag and refuses ... on an existing name at EVERY
  scope"). If a caller runs `--apply` with a URL/auth combination that
  ALREADY has an unreproducible custom-header registration in place, "detect
  it, report preserved, never replace it" and "converge because that's what
  `--apply` with a URL always does for claude-code" are contradictory
  instructions to the SAME code path — and the shipped remove step has NO
  way to distinguish "the existing registration is one we should preserve"
  from "the existing registration is stale and should be cleared" without
  the drift comparison actually gating whether the remove-then-add sequence
  even runs. Silently choosing "converge always wins" reproduces the
  2026-09-10 incident's exact mechanism for claude-code, since the remove
  step has always run unconditionally to date.

**Why it happens:**
The two requirements were written by the same milestone for good reasons —
idempotent re-run and non-destructive reconcile are both real goals — but
neither requirement, as scoped, states which one governs when they conflict
on the ONE runtime whose convergence mechanism (remove-then-add) is itself
destructive.

**How to avoid:**
Resolve this as an explicit, written decision, not an implicit code
consequence: reconcile's "preserve" must be checked BEFORE
`claudeCodeRemoveAction` is scheduled at all, for every apply — i.e., drift
detection is not merely a preview-time feature (Pitfall 1) but the actual
GATE that decides whether claude-code's write sequence runs, is skipped
with a reported "preserved, not converged" outcome, or proceeds normally
because the existing registration IS one setup wrote and can safely be
replaced. This makes "preserved" a real THIRD outcome alongside
`OutcomeWrote`/`OutcomeAlreadyCorrect` for claude-code specifically, not a
preview-only annotation — a new `Outcome` value (`plan.go`'s five-value enum
already documents that "every code path ... must set one of the five
constants" — this is exactly the kind of change that enum's own philosophy
anticipates needing a sixth, explicit value for, never an implicit fallback
onto an existing one).

**Warning signs:** A fixture where an existing claude-code registration
carries a header the new drift comparison flags as unreproducible, followed
by `--apply` — if the resulting `Result.Outcome` is `OutcomeWrote` (or the
`claude mcp remove` action ran at all), the collision was resolved in favor
of the wrong requirement.

**Phase to address:** Drift-detection/reconcile phase, in explicit
coordination with the claude-code sub-task — this is the single riskiest
integration point in the whole milestone, since claude-code is also the
runtime whose OAuth re-login cost (see Pitfall 6) makes an unnecessary
remove-then-add cycle expensive even when it does succeed.

---

### Pitfall 6: A "successful" reconcile that still runs remove-then-add on an OAuth-authenticated Claude Code registration forces re-login for no observable reason

**What goes wrong:**
`docs-site/guides/agent-setup.md` already states, as a known property of
the existing design: "Registration and OAuth login are separate steps:
complete the runtime's OAuth login after successful registration." Claude
Code's OAuth token is tied to the registration entry `claude mcp remove`
clears — PROJECT.md's own incident summary confirms this happened for real:
"Claude Code needed re-auth after `mcp remove` + `mcp add`." If drift
detection determines a claude-code registration is "already correct" in
every respect EXCEPT some field the comparison logic considers changed
(e.g., a normalization difference in how the URL or an unrelated header is
rendered — see Pitfall 7), and reconcile decides to "converge" by running
the existing remove-then-add sequence anyway, the operator experiences an
unprompted OAuth re-login for a registration that, from their perspective,
was not meaningfully different — with no drift-detection message explaining
WHY a re-login was suddenly required, since the existing report shape
(`Result.Reason`/`Result.Notes`) was designed for failure/tolerance
narration, not for "this write was necessary because X differed."

**Why it happens:**
Drift comparison is naturally implemented as a byte- or field-level
inequality check; ANY field-level difference currently maps to "not already
correct" under the shipped `execute()` model (`D-08`'s byte-compare is
binary — identical or not, with no notion of "differs in a way that doesn't
matter"). Converting "differs" into "requires an OAuth-costly rewrite" is an
implicit consequence of reusing the existing convergence path, not a
decision anyone makes explicitly.

**How to avoid:**
Treat "this field's drift is real and warrants a rewrite" and "this field's
drift is cosmetic/probe-artifact and should be ignored" as a decision the
new comparison logic must make explicitly per field (URL normalization,
header ordering, whitespace) — reusing Pitfall 7's normalization work — and
surface, in the reported outcome, WHY a claude-code rewrite is about to
force re-login, so an operator can decide to defer `--apply` rather than
being surprised. At minimum, the generated report/prose should state the
re-login consequence explicitly whenever claude-code's remove-then-add path
is about to run for a registration that was OAuth-authenticated — mirroring
`claudeCodeRemoveAction`'s existing Description discipline (surfacing a
consequence via `Result.Notes` even when the step itself succeeds).

**Warning signs:** A drift-comparison fixture where only whitespace or key
ordering differs between the stored and requested registration, and the
resulting outcome is still "would rewrite" for claude-code with no
distinguishing note from a genuine credential/URL change.

**Phase to address:** Drift-detection/reconcile phase, claude-code sub-task
— pair directly with Pitfall 5's gating decision, since both concern the
same remove-then-add sequence.

---

### Pitfall 7: Comparing a normalized registration against a runtime whose OWN read verb is lossy or non-deterministic — `opencode mcp list` is the sharpest case, but not the only one

**What goes wrong:**
`opencode.go`'s own extensive doc comment already states the core problem
for the SHIPPED byte-compare (D-08): `opencode mcp list` is "the ONLY read
verb available," it "renders a human-formatted table with box-drawing and
status glyphs," it "lists EVERY registered MCP server (not just engram's),"
and it "dials the network for each of them on every invocation" — so two
reads of the SAME unchanged state can differ for reasons entirely unrelated
to engram's own registration (another server's transient connection status
flipping a status glyph). The shipped code's answer is D-08's own escape
hatch: ambiguity resolves to `OutcomeWrote`, never `OutcomeAlreadyCorrect,`
which is SAFE for the binary "did this converge" question but is NOT an
answer for the NEW question drift detection asks: "does the EXISTING
registration's URL/auth-shape/header-set match what setup would write."
Building drift detection's comparison on top of the SAME `mcp list` output
means:
1. **False-positive drift.** An unrelated server's status glyph, or the
   the table's own column-width padding (which can shift when another
   server's name is longer or shorter), changes the RAW captured bytes
   between two runs, which a naive full-registration-string comparison
   would report as "the registration drifted" even though engram's own
   entry did not change at all.
2. **False-negative preserve.** Conversely, `mcp list`'s table format may
   not expose enough structure to reliably ISOLATE engram's own row's
   header set from the rest of the table at all — parsing a third-party
   human-formatted table to extract a structured comparison target is
   EXACTLY the kind of "scraping a format that drifts as easily as the
   flag surface it claims to protect against" this package's own
   `apply.go` doc comment already rejects for a different purpose
   ("no pre-flight probe of any runtime's `--help` output ... matching
   tokens in help text is scraping a format that drifts").
3. A parallel, less severe version of the same problem exists for Codex:
   `codex mcp get <name> --json` DOES return structured JSON (confirmed
   live this session: `--json` flag exists and is documented as
   "Output the server configuration as JSON") — a genuinely reliable
   comparison target — but Claude Code's `claude mcp get <name>` has NO
   `--json` flag at all (confirmed live this session: `claude mcp get
   --help` lists only `-h/--help`), so its comparison target is
   necessarily the SAME kind of human-formatted text `apply.go` already
   treats as an ambiguity-resolves-safely case for the narrower
   byte-compare, not a green light for a NEW structured-diff feature to
   assume the same text is parseable into fields.

**Why it happens:**
The shipped code already solved the NARROW problem (converge-or-not) by
choosing to be conservative rather than parse anything — it never needed a
STRUCTURED comparison. Drift detection's stated design ("preview compares
the full existing registration ... URL, auth shape, header set") is asking
for exactly the structured extraction the shipped design avoided, and it is
easy to reach for "just parse the same output we already read" without
re-deriving why that output was never trusted for structure before.

**How to avoid:**
For each runtime, decide explicitly which comparison granularity its OWN
read verb can honestly support, and record that as a per-runtime fact
(mirroring D-09's "authored here, in the runtime's own file" discipline):
Codex's `--json` output is the one case where a genuinely structured,
field-level comparison is honest; Claude Code's and opencode's text output
should drive a COARSER, more conservative comparison (e.g., "does the whole
captured text change" — closer to the existing D-08 shape — rather than
"does the extracted header field change"), with any apparent drift on those
two runtimes defaulting toward "cannot confidently determine, report as
preserved/unknown" rather than confidently misreporting either false
positive or false negative. Never write a parser for `opencode mcp list`'s
box-drawing table to extract engram's own row — if opencode's comparison
needs more structure than the raw text safely provides, that is a scope
boundary to state explicitly ("opencode drift detection is best-effort /
whole-table-change only"), not a parsing problem to solve.

**Warning signs:** A drift-detection fixture with an unrelated second MCP
server present alongside engram's own entry reports drift on engram's
registration when only the OTHER server's state changed.

**Phase to address:** Drift-detection/reconcile phase — decide and record
per-runtime comparison granularity as an explicit early task, before writing
any comparison logic, since it changes what the comparison function's
return type even needs to express (a boolean "matches" vs. a structured
field-level diff).

---

### Pitfall 8: Plugin install/marketplace-add without consent — and this maintainer's own machine is ALREADY in the exact partial state that makes the consent question real

**What goes wrong:**
Live-inspected this session, read-only, on the machine this research ran on:
`claude plugin marketplace list` already shows an `engram` marketplace
configured (source: GitHub `seanb4t/engram`), and
`~/.claude/plugins/marketplaces/engram` and
`~/.claude/plugins/cache/engram/engram` both exist on disk — but
`~/.claude/plugins/installed_plugins.json` shows **no engram entry actually
installed**, and no `~/.claude/skills/curating-memory` (or any other engram
skill) directory exists under the plain-install path either. This is a
REAL, currently-live "marketplace added, plugin not installed, no plain
skills present" state — not a hypothetical fixture — and it demonstrates
exactly the ambiguity a plugin-first `--apply` must resolve correctly:
1. **Marketplace-add is itself a trust decision.** `claude plugin
   marketplace add <source>` fetches and caches a marketplace definition
   from a URL/GitHub repo BEFORE any plugin is installed from it — running
   this unconditionally under `--apply` (with no separate confirmation)
   adds a new trust surface (a marketplace source, `seanb4t/engram`'s own
   `.claude-plugin/marketplace.json` if one exists, or the repo itself)
   that a `bearer`/`none`-auth, no-plugin-opinion user of today's
   `engram setup` never had to accept. The existing binary-setup docs
   explicitly draw the line the other way already: "Binary setup does not
   install the standalone Claude plugin's session hooks" — plugin-first
   delivery being scoped for `--apply` under the SAME command inverts that
   documented boundary, and the docs/consent UX must be updated
   deliberately, not left stale (a stale doc here is worse than usual,
   since it directly contradicts the new shipped behavior).
2. **`claude plugin install` and `claude plugin update` both support
   `--json`/`-y` (`--yes`) flags** (confirmed live this session): `-y`
   "Accept the displayed marketplace-declared command without the
   confirmation prompt ... required when stdin or stdout is not a TTY."
   This means a scripted, non-interactive `engram setup --apply` MUST pass
   `-y` for plugin install to succeed non-interactively at all — which
   means engram's own `--apply` invocation, not a human, becomes the thing
   that accepts "the displayed marketplace-declared command" sight-unseen
   on the operator's behalf. That is a materially bigger consent
   surface than anything `engram setup` has shipped to date (every prior
   write was a single, fully-specified `mcp add`/`mcp remove` invocation
   engram itself authored and displayed in preview — a plugin's declared
   install command is AUTHORED BY THE MARKETPLACE, not by engram, and
   engram's own preview cannot show it without first resolving the
   marketplace, which itself has side effects per point 1).
3. **Codex's plugin surface is structurally the same shape**
   (`codex plugin add <PLUGIN[@MARKETPLACE]>`, confirmed live this
   session) — `codex plugin marketplace add` similarly requires trusting a
   marketplace source before `codex plugin add` can resolve `PLUGIN@engram`.

**Why it happens:**
"Plugin-first" reads, at the requirements level, as a pure DELIVERY
mechanism change (skills/hooks/command ship via a different channel) — but
operationally it is also a NEW trust-and-consent surface (a marketplace
source, plus a marketplace-declared install command neither engram nor the
operator authored) layered underneath a command (`--apply`) whose entire
prior design assumed every write action's exact argv was authored and
previewable by engram itself.

**How to avoid:**
Preview MUST show, in full, both the exact `marketplace add`/`plugin
install` invocations AND — where the CLI supports it (`claude plugin
install --json`, `claude plugin details <name>` per the `--help` output
captured this session) — the marketplace-declared command that install will
run, BEFORE `--apply` ever passes `-y`/accepts it non-interactively. Decide
explicitly whether a first-run plugin install needs an EXTRA, separate
opt-in beyond the existing `--apply` flag (a `--allow-plugin-install` shape,
or equivalent) given that the marketplace source and declared command are
not engram's own authored content the way every prior `Action.Args` was —
and record that decision, since the milestone's own PROJECT.md is silent on
consent UX specifically. Test against the REAL partial state this session
found live (marketplace present, plugin not installed) as an explicit
fixture, not just the two clean-slate cases (nothing present / fully
installed) — the partial state may be common precisely because prior
research/experimentation (like this milestone's own predecessor work)
leaves exactly this residue.

**Warning signs:** `engram setup --apply` for claude-code silently performs
`marketplace add` before the operator has seen the marketplace source URL
in preview output; a scripted/CI `--apply` invocation requires discovering
`-y` semantics by trial and error rather than from engram's own `--help`.

**Phase to address:** Plugin-first-delivery phase — the consent/preview
design here is a prerequisite decision, not an implementation detail, given
this session found the "marketplace present, plugin absent" ambiguity is
not hypothetical.

---

### Pitfall 9: Plain-install and plugin-install can double-register the same skill under two different names, and a naive migration can also silently orphan the OLD copy or misuse a stow/chezmoi symlink

**What goes wrong:**
Two related risks, both grounded in shipped code:
1. **Double registration.** `internal/skills/install.go`'s `installFiles`
   writes each skill at `filepath.Join(dir, s.Name, f.Path)` — for
   claude-code today, `~/.claude/skills/curating-memory/SKILL.md`. Claude
   Code's OWN plugin system separately names an installed plugin's skills
   with a `<plugin>:<skill>` prefix in its own UI/skill-listing (the
   milestone's own scoping text names this exact collision: "duplicate
   `curating-memory` next to the plugin's `engram:curating-memory`"). If
   plugin-first delivery for claude-code is added WITHOUT also making the
   plain-install path a no-op for claude-code specifically, a machine that
   runs `--apply` after this milestone ships gets BOTH: the plugin's
   `engram:curating-memory` (from the marketplace/plugin flow) AND a plain
   `~/.claude/skills/curating-memory` directory (from the pre-existing,
   still-unconditional `claudeCodeRuntime.Plan`'s `SkillTarget{Format:
   SkillFormatNative, Dir: filepath.Join(home, ".claude", "skills")}`) —
   this is not a hypothetical drift scenario, it is the CURRENT shipped
   code path, unconditionally executed, that this milestone's PROJECT.md
   explicitly names as the thing that must change: "Today `internal/skills/`
   and `internal/setup/` have no plugin awareness: on the maintainer's
   machine `--apply` would write a duplicate `curating-memory` next to the
   plugin's `engram:curating-memory`."
2. **Migration orphan / symlink-replacement risk on cleanup.** The natural
   fix for (1) — once claude-code is plugin-first, have `--apply` DELETE
   the stale plain `~/.claude/skills/curating-memory/` directory a PRIOR
   binary-setup run may have left behind — introduces a NEW write mode
   (delete) this package has never had (every existing skills-facet write
   is additive-or-overwrite per `installFiles`'s byte-compare-then-write,
   never a delete). If that cleanup naively does `os.RemoveAll` on a path
   that is itself a symlink into a chezmoi/yadm/stow-managed dotfiles repo
   (the exact scenario `internal/skills/install.go`'s own `WriteFile`
   doc-comment for `installAgentsMDIndex` already reasons carefully about
   for a DIFFERENT file: "this repository's own AGENTS.md is one such
   symlink" and os.WriteFile-through-a-symlink vs.
   stage-and-rename-replaces-the-symlink), a delete-based cleanup could
   remove the operator's own dotfiles-managed skill source, not just
   engram's copy of it — a strictly worse failure mode than merely leaving
   a stale duplicate.
3. PROJECT.md's own text ALSO names the symlink-replacement risk directly
   for a different artifact: "replace Codex's marketplace symlinks with
   static copies pinned to the binary's embedded version" — i.e., Codex's
   OWN plugin caching mechanism uses symlinks (confirmed structurally this
   session: `~/.claude/plugins/marketplaces/engram` and
   `~/.claude/plugins/cache/engram/engram` both exist as the marketplace's
   own managed structure) that a plain, binary-embedded skills copy
   running AFTER plugin install could silently overwrite with static files
   pinned to whatever version shipped inside that particular `engram`
   binary — regressing a marketplace-tracked (auto-updatable) skill to a
   binary-pinned (stale-on-next-plugin-update) one, with no visible error.

**Why it happens:**
The plain-install path was built and hardened (byte-compare idempotency,
symlink-preserving `AGENTS.md` writes) BEFORE any plugin awareness existed
— it is correct in isolation. Plugin-first delivery is a routing decision
layered on top ("for this runtime, use the plugin path INSTEAD"), and
"instead" silently implies "and therefore skip/undo the other path," which
is new logic nothing in the shipped code currently performs — the
`FormatNone`/`FormatNative`/`FormatAgentsMD` exhaustive switch in
`skills.Install` has no fourth case for "this runtime is plugin-managed,
skip skills entirely," and `setupSkillsTarget`'s exhaustive mapping in
`cmd/engram/setup.go` would need a new case too, with the SAME
never-silently-coerce discipline it already applies to an unrecognized
format.

**How to avoid:**
Route claude-code (and codex, once its plugin path lands) to
`SkillFormatNone` — the EXISTING no-op value the generic pseudo-runtime
already uses — the moment plugin-first delivery is selected for that
runtime, rather than inventing new delete logic. This turns "stop writing
the plain copy" into a change that reuses an already-tested code path
(`skills.Install`'s `FormatNone` case: "has nothing to write and returns
immediately") instead of adding destructive new behavior. For the SEPARATE
question of a stale plain copy left over from a PRE-plugin-first version of
engram: report its presence explicitly (a new, additive check — "found a
plain skill install at X that plugin-first delivery no longer manages;
remove it yourself if you want to" — surfaced via `Result.Notes`, matching
the existing tolerant-action-Notes discipline) rather than deleting it
automatically. Never `os.RemoveAll` a path without first confirming (via
`os.Lstat`, not `os.Stat`) that it is not itself a symlink the operator's
own dotfiles tooling manages — mirroring the reasoning
`installAgentsMDIndex`'s own doc comment already applies to writes, extended
to the NEW case of deletes.

**Warning signs:** A fixture with a pre-existing `~/.claude/skills/
curating-memory/SKILL.md` (simulating a pre-plugin-first install) shows BOTH
that directory and a successful plugin install after `--apply` runs on the
post-plugin-first binary; a cleanup step that follows a symlink and deletes
through it rather than stopping at the link.

**Phase to address:** Plugin-first-delivery phase — the `SkillFormatNone`
routing decision is small and should land with the plugin-path decision
itself; the stale-copy detection is a small additive check that should not
be deferred, since it is the DIRECTLY NAMED failure mode in this milestone's
own PROJECT.md.

---

### Pitfall 10: Hand-edits in Codex's `config.toml` that `codex mcp add` would clobber — "reconcile hand-edits" needs a source Codex's own CLI cannot give it

**What goes wrong:**
`codex.go`'s own doc comment already records the load-bearing fact this
pitfall turns on: "Codex is the only registered runtime whose `mcp add`
genuinely overwrites an existing entry silently" — confirmed structurally
this session (`codex mcp add --help` shows no `--force`/confirmation gate
at all; it simply writes). This is DIFFERENT from claude-code's refuse-on-
existing behavior and opencode's (undocumented-either-way) behavior, and it
is the reason `codexRuntime.Plan` needs no remove-then-add sequence today —
but it is ALSO exactly the mechanism that makes "reconcile hand-edits"
structurally harder for Codex than for the other two runtimes: if an
operator hand-edited `~/.codex/config.toml`'s `[mcp_servers.engram]` table
directly (adding a comment, a field engram doesn't know about, or a
provider-specific extension key), `codex mcp add engram --url <...>` does
not read, merge, or preserve that hand-edited table — it silently
overwrites the whole entry the moment `--apply` runs, REGARDLESS of what
drift detection concluded, because Codex exposes NO in-place-update-only
primitive and no dry-run/diff flag on `mcp add` itself (confirmed: no such
flag in the `--help` output captured this session). Drift detection can
tell the operator "this differs from what I would write" via `codex mcp get
--json` (a reliable, structured read — Pitfall 7's one genuinely-honest
comparison target), but "detected != preserved" for Codex specifically,
because there is no Codex-native write path that ONLY updates the fields
engram cares about while leaving unknown TOML keys inside that one table
untouched — the only tools are "overwrite the whole entry" (`mcp add`) or
"read-only" (`mcp get`).

**Why it happens:**
"Reconcile hand-edits" implicitly assumes a merge or partial-update
capability exists somewhere in the toolchain being reconciled against; for
Codex, the shipped design deliberately shells out to `codex mcp add` rather
than hand-writing TOML (the prior milestone's Pitfall 6 already established
why: zero-new-deps, no comment/ordering-preserving TOML round-trip in
stdlib) — which means engram has NO write primitive of its own that could
implement a partial merge even if it wanted to; it is entirely dependent on
whatever `codex mcp add`'s own semantics happen to be.

**How to avoid:**
For Codex, "preserve a hand-edited registration" can only mean "detect that
one exists (via `codex mcp get --json`'s reliable structured read) and
REFUSE to run `mcp add` at all for that runtime, reporting the hand-edited
state as preserved-by-non-action" — never "merge the hand-edit into the new
write," since no primitive exists to do that merge. This is a materially
different resolution than claude-code's (Pitfall 5's gate-before-remove) or
opencode's (Pitfall 7's coarse-comparison) cases, and should be recorded
explicitly as a Codex-specific limitation in whatever design doc or
docs-site update this phase produces — an operator hand-editing
`config.toml` for Codex, unlike an operator hand-editing nothing, needs to
understand that `engram setup --apply` for Codex will EITHER skip Codex
entirely (preserving the hand-edit) OR overwrite it whole (destroying the
hand-edit) — there is no partial-preserve middle ground the CLI can offer.

**Warning signs:** A fixture with a `[mcp_servers.engram]` table carrying an
unrecognized extra key (simulating a hand-edit) followed by `--apply` shows
that key gone from the resulting registration — proving `mcp add`'s
overwrite-whole-entry behavior actually destroyed the hand-edit rather than
merely reporting it.

**Phase to address:** Drift-detection/reconcile phase, Codex sub-task —
this needs its own explicit written decision (skip-whole-runtime vs.
overwrite-whole-runtime, no partial option), distinct from claude-code's and
opencode's resolutions, and should be the FIRST Codex reconcile fixture
written since it is the runtime with the least flexible CLI.

---

### Pitfall 11: Manpage generation regresses the completions design this project ALREADY fixed once — and the milestone's own scoping text is stale about how completions ship today

**What goes wrong:**
PROJECT.md's current milestone scope states: "The cask's
`generate_completions_from_executable` hook already expects a completion
verb." **This is factually stale relative to the shipped `.goreleaser.yaml`**
(read directly this session, lines ~178–200): the cask's `post.install` hook
explicitly and deliberately does NOT use Homebrew's
`generate_completions_from_executable` helper at all — it hand-writes
completions via `system_command binary, args: ["completion", shell]` for
each of bash/zsh/fish, with its own comment explaining exactly why: "never
via Homebrew's completion-generation helper — that helper's `write_completion`
wraps execution in a rescue that downgrades a failure to a warning, so a
broken binary would install green." `cmd/engram/releaseconfig_test.go`'s
`TestReleaseConfigCaskInstallGate` enforces this as a hard acceptance gate:
it asserts the string `generate_completions_from_executable` occurs **zero**
times anywhere in `.goreleaser.yaml`, including comments — the test's own
comment explains why even a comment mentioning the helper's name is
forbidden: "the acceptance gate for this decision is a literal occurrence
count over this file, so naming it even in a comment destroys the gate's
ability to tell prose from actual use." Three concrete risks follow for the
manpage work:
1. **Following the milestone's stale scoping text literally** (wiring into
   `generate_completions_from_executable`, or even mentioning it in a new
   comment while explaining why manpages work differently) would either
   reintroduce the exact silent-failure trap this project already
   diagnosed and fixed (prior milestone's Pitfall 3/4: `rescue`-wraps
   execution, "a warning, never a raise"), or fail the existing acceptance
   test outright the moment a PR touches `.goreleaser.yaml` near that
   block.
2. **cobra/doc's `GenManTree` is a Go API, not a CLI subcommand** — unlike
   `completion`, which cobra auto-registers as a real subcommand
   (`cmd/engram/testdata/help.golden` already lists it), there is no
   built-in `engram man`-shaped verb for a cask postflight hook to exec the
   way it execs `engram completion <shell>` today. Reusing the EXACT same
   "generate from the installed binary via `system_command`" pattern for
   manpages requires FIRST adding a new hidden Cobra command in
   `cmd/engram` that calls `doc.GenManTree` internally — this is new
   surface area the milestone's "zero new Go dependencies" framing
   undersells: `cobra/doc` being "already an indirect dependency" (true,
   per `go.mod`'s own comment: "transitively today by cobra/doc and buf —
   no new module is fetched") means no NEW module needs fetching, but the
   dependency still needs PROMOTING from indirect to direct in `go.mod`
   (this repo's own precedent: `go.yaml.in/yaml/v3` was promoted the same
   way for skill frontmatter in the prior milestone) — skipping that
   promotion risks a `go mod tidy` drift check failing in CI the moment
   the import is added without the corresponding `go.mod` edit.
3. **cobra/doc's generated output is non-deterministic by default.** Cobra
   inserts an "Autogenerated by spf13/cobra" timestamp line into both
   completion scripts and `GenManTree` output unless the command tree sets
   `DisableAutoGenTag = true` — this repo relies HEAVILY on golden-file
   tests (`help.golden`, and the pattern `TestReleaseConfigCaskInstallGate`
   itself exemplifies) for exactly this class of generated-content
   determinism; a manpage-generation golden test (or even just a rehearsal
   run compared byte-for-byte across two invocations) will be spuriously
   flaky/non-reproducible if this flag is left at its default.

**Why it happens:**
The milestone's own PROJECT.md was written from an earlier understanding of
how completions ship (or the phrasing is imprecise shorthand for "the cask
already has an install-time completions mechanism") — but a phase plan that
trusts that sentence literally, rather than re-reading `.goreleaser.yaml`
directly, will build the wrong thing. Separately, cobra's own defaults
(auto-gen timestamp) are easy to overlook because they only matter once
something diffs the generated output across two runs, which a first "does
it produce a man page" smoke test would not surface.

**How to avoid:**
Re-verify `.goreleaser.yaml`'s actual completions mechanism directly (as
this research did) before writing the phase plan or requirements text for
manpages — do not propagate PROJECT.md's `generate_completions_from_
executable` phrasing into code, comments, or a new test without first
confirming it against the file. Design manpage generation to MIRROR the
completions pattern's actual shape (a new hidden cobra command, exec'd from
the SAME cask postflight block, AFTER the version-assertion gate — the
existing `checkOrdering` test's third assertion, `"version", "--output",
"json"` before `args: ["completion"`, should gain a parallel assertion for
whatever the man verb's marker string is), not the helper it deliberately
avoids. Set `RootCmd.DisableAutoGenTag = true` before calling `GenManTree`
(and confirm `GenBashCompletion`'s auto-gen tag setting is already handled
the same way, if not already verified) so any generated-content comparison
test is deterministic. Promote `cobra/doc` from indirect to direct in
`go.mod` in the SAME commit that first imports it, following the
`go.yaml.in/yaml/v3` precedent exactly.

**Warning signs:** A grep for `generate_completions_from_executable`
anywhere in a manpage-phase PR's diff (including comments) — the existing
test already fails loudly on this, so this is more a "catch it before CI"
warning than a hidden risk, but the failure mode is worth naming since the
milestone's OWN scoping text points the wrong way. Separately: two
consecutive local `engram man`-equivalent generation runs producing
byte-different output (the auto-gen-tag timestamp) is the manpage-specific
non-determinism signature.

**Phase to address:** Completions/manpages phase — re-verify the actual
`.goreleaser.yaml` mechanism as the FIRST task, before any code or
requirements text is written from PROJECT.md's summary of it.

---

### Pitfall 12: A new hidden `man`-generation command needs the SAME exclusion discipline `completion` already has, or it silently pollutes every surface-conformance and catalog test

**What goes wrong:**
`cmd/engram/cmdwalk.go` already carries a narrow, explicit exclusion:
"cobra's own `help`/`completion` scaffolding (auto-registered ... ) ... is
Hidden or its Name() is `help` or `completion`" (`isSkipped`, referenced in
the doc-comment excerpt captured this session), and multiple tests
(`cmdwalk_test.go`, `surfaces_test.go`, `golden_test.go`) depend on that
exact, closed enumeration to keep `--help` output, the operator-command
catalog (`catalog.go`), and the `internal/surfaces` conformance gate
(`v0.13.x`'s "declare each conditional rule once, derive presence-checking
across five surfaces") stable. A new hidden command added for manpage
generation (whatever it is named — `man`, `gendoc`, `docs`) is, BY
CONSTRUCTION, a sixth cobra command sibling to `completion` — but nothing in
`isSkipped`'s current three-way check (`cmd.Hidden`, `Name() == "help"`,
`Name() == "completion"`) will exclude it automatically. If it is added
without `Hidden: true` AND without extending the skip predicate, it will
appear in `Names()`/the operator catalog/`--help` golden output as a
real, user-facing command — breaking `help.golden` and any exhaustive
"every command has X" surfaces conformance check the moment it is added,
in a way that is easy to chase as an unrelated regression rather than
recognize as "a new hidden command needs the same treatment as
`completion`."

**Why it happens:**
`completion`'s exclusion was hand-coded for a SPECIFIC cobra auto-registered
name, not as a general "any hidden doc-generation utility" rule — adding a
structurally similar but differently-named command doesn't inherit that
treatment just because it serves an analogous purpose.

**How to avoid:**
Either (a) mark the new command `Hidden: true` (which `isSkipped` already
honors regardless of name — the OR-condition `cmd.Hidden || Name() ==
"help" || Name() == "completion"` covers any hidden command generically),
which is the simpler and more future-proof choice, or (b) if it must be
visible for some reason, extend `isSkipped`'s three-way check explicitly
and update every test that enumerates the excluded set by name
(`cmdwalk_test.go` at minimum). Prefer (a): mirror `completion`'s own
"hidden utility, not a first-class user command" positioning rather than
adding a fourth named exception to a check whose own doc comment implies a
short, closed list.

**Warning signs:** `help.golden` (or any `nonHiddenCommands`-driven test)
fails immediately after the new command is added, listing it as an
unexpected addition.

**Phase to address:** Completions/manpages phase — mark the command Hidden
from its first commit, verified by running the existing golden tests
(`go test ./cmd/engram/... -run Golden`) before considering the task done,
not as an afterthought once a test happens to fail.

---

### Pitfall 13: `#560`'s `ctx.Err()` fix looks small in isolation but changes the observable contract every OTHER pitfall's fixture tests rely on

**What goes wrong:**
`environment.go`'s `osRun` currently converts ANY `*exec.ExitError` —
including one produced by `exec.CommandContext`'s own deadline-triggered
kill — into `RunResult{ExitCode: exitErr.ExitCode()}` with a **nil** error,
never consulting `ctx.Err()` to distinguish "the process ran and exited
nonzero on its own" from "the process was killed because the context
deadline expired." Every pitfall in THIS document that proposes a new
timeout-sensitive behavior (Pitfall 4's live-verification probes, Pitfall 7's
per-runtime read-verb reliability, any new plugin-install `Environment.Run`
call which may legitimately take longer than `execTimeout`'s existing 20s
constant given a real network fetch from a marketplace) will be built and
tested against the CURRENT, buggy contract unless this fix lands FIRST or
concurrently — a fixture test asserting "a plugin install that times out
reports a distinguishable timeout reason" cannot be written correctly
against the current `osRun`, since a deadline-killed process today reports
as an ordinary nonzero exit (frequently exit -1 on Unix for a SIGKILL'd
process), indistinguishable in the `RunResult`/`Result.Reason` shape from a
genuine CLI usage error.

**Why it happens:**
The bug is narrow and easy to treat as a pure cleanup item ("W01 — fix the
exit-code conversion") independent of the new features — but plugin
install is the first NEW call site in this milestone plausibly slow enough
(network fetch of a marketplace/plugin archive) to actually HIT
`execTimeout` in practice, where every existing call site (`mcp add`/`mcp
get`, all under ~2s per the shipped research) essentially never did.

**How to avoid:**
Land the `#560` fix (consult `ctx.Err()` in `osRun`'s error-classification
switch — a `context.DeadlineExceeded`/`context.Canceled` check alongside the
existing `errors.As(runErr, &exitErr)` branch) BEFORE or ALONGSIDE the
plugin-install phase, not as an independent, later cleanup — and write the
plugin-install timeout fixture test against the FIXED contract, asserting a
distinguishable timeout reason (not a bare nonzero-exit `Reason` string)
reaches the operator. Consider, explicitly, whether plugin install's likely
longer network latency means `execTimeout`'s existing fixed 20s constant
(documented as deliberately non-tunable because "every runtime's `mcp
add`/`mcp get` surface completed in under 2 seconds") needs its own
per-call-site override now that a genuinely slower operation exists — this
is the kind of the "not yet earned" tunability that constant's own comment
already anticipates revisiting.

**Warning signs:** A plugin-install timeout in the wild reports as a bare
"exited -1" (or similar) rather than a legible timeout message; a fixture
test for plugin-install timeout handling can only be written by asserting
on exit code -1 rather than on a distinct seam-error path.

**Phase to address:** This is explicitly carried as its own bullet in
PROJECT.md ("#560 `osRun` deadline classification") — sequence it before or
alongside the plugin-first-delivery phase specifically, since that is the
first phase whose new `Environment.Run` call sites make the bug
practically reachable rather than theoretical.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|-----------------|-----------------|
| Reusing `--bearer-token-env-var` on Codex for any custom-header request that merely "looks token-shaped" | No new Codex-specific unsupported-mode branch to write | Silently writes the wrong header NAME (`Authorization` instead of the requested one), reporting `wrote` for a registration that will fail auth (Pitfall 2) | Never — only literal `Authorization: Bearer <ref>` custom-header requests may reuse it |
| A shared cross-runtime header-formatting helper | Less code, one place to fix a bug | Reproduces the exact opencode `KEY=VALUE` vs `KEY: VALUE` regression this project already fixed once, for the new code path (Pitfall 3) | Never — keep header rendering authored per-runtime, matching the existing `AUTHORED HERE (D-09)` discipline |
| Treating drift detection as sufficient protection against `--apply` overwriting a real registration | Ships the "preview shows preserved" requirement text quickly | Does nothing to stop the SAME unconditional write sequence that caused the 2026-09-10 incident, since preview and apply are different code paths today (Pitfall 1) | Never, given this is the literal incident the milestone exists to prevent |
| Auto-deleting a stale plain-install skill directory once plugin-first ships for that runtime | Cleans up the exact duplicate-skill state PROJECT.md names | `os.RemoveAll` on a path that may be a chezmoi/yadm/stow-managed symlink destroys the operator's own dotfiles source, not just engram's copy (Pitfall 9) | Never automatically — report the stale copy, let the operator remove it |
| Parsing `opencode mcp list`'s human-formatted table to extract a structured drift comparison | Gets opencode to the same comparison granularity as Codex's `--json` read | Third-party output-format scraping this package's own `apply.go` doc comment already rejects for a different purpose; breaks silently on unrelated servers' status changes (Pitfall 7) | Never — keep opencode's comparison coarse/whole-text, or explicitly best-effort |
| Deferring the `#560` `ctx.Err()` fix as unrelated cleanup, independent of plugin-install | Smaller, more focused PR for the timeout fix alone | Plugin-install's fixture tests get built against the CURRENT buggy timeout-classification contract and need rework once the fix lands anyway (Pitfall 13) | Only if plugin-install's own timeout-handling tests are written AFTER the fix lands, never before |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|-----------------|-------------------|
| Custom-header auth ↔ Codex's `mcp add` | Assuming a "bearer token env var" flag can express an arbitrary header name | Route any non-`Authorization` custom header to the unsupported/preserve path for Codex specifically (Pitfall 2) |
| Custom-header auth ↔ opencode's `--header` | Assuming the SAME `NAME: VALUE` rendering that works for claude-code also works for opencode | Author opencode's header rendering with its own `NAME=VALUE` separator, in `opencode.go` only, never a shared formatter (Pitfall 3) |
| Drift detection ↔ each runtime's read verb | Treating `claude mcp get`/`opencode mcp list`/`codex mcp get --json` as equally structured comparison sources | Grant Codex's `--json` output field-level comparison; treat claude-code's and opencode's text output as coarse/whole-text only (Pitfall 7) |
| Reconcile ("preserve hand-edits") ↔ Codex's silent-overwrite `mcp add` | Assuming "preserve" can mean "merge the hand-edit into the new write" | For Codex, preserve can only mean "skip the whole runtime's write," since no partial-update primitive exists (Pitfall 10) |
| Plugin-first delivery ↔ existing plain-install skills path | Leaving `claudeCodeRuntime.Plan`'s `SkillFormatNative` skills target wired unconditionally once plugin delivery is added | Route plugin-managed runtimes to the existing `SkillFormatNone` no-op value; report (never auto-delete) any stale plain copy (Pitfall 9) |
| Manpage generation ↔ the cask's existing completions mechanism | Wiring into or mentioning Homebrew's `generate_completions_from_executable` helper, per PROJECT.md's stale phrasing | Mirror the ALREADY-SHIPPED `system_command binary, args: ["completion", shell]` pattern with a new hidden cobra command for manpages, never the helper the acceptance test forbids naming (Pitfall 11) |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Rendering a drift-detection "header set" field from a probe's raw captured output without first confirming that runtime's read verb only ever echoes an unresolved reference | A resolved secret value reaches `--output json`, a CI log, or a committed generated markdown file via `internal/setupgen` (Pitfall 4) | Live-verify each read verb's echo behavior for a non-bare-reference header value BEFORE trusting its output in any rendered field; sanitize/bound exactly as `apply.go`'s existing `maxCapturedBytes` discipline already does for other captures |
| Accepting a marketplace-declared plugin-install command non-interactively (`-y`) inside `--apply` without first previewing its exact content | `--apply` executes an install command authored by a third-party marketplace, not by engram, with no prior operator visibility — a materially larger trust surface than any prior `Action.Args` engram itself authored | Preview the marketplace source and the declared install command explicitly before `--apply` ever passes `-y`/accepts it (Pitfall 8) |
| Auto-deleting a file/directory that turns out to be a dotfiles-managed symlink, in service of the new plugin-vs-plain-install cleanup | Silently destroys the operator's own dotfiles repository content, not just engram's managed copy | `os.Lstat` (never `os.Stat`) before any delete; report stale plain-install copies rather than removing them automatically (Pitfall 9) |
| Running `codex mcp add`/`claude mcp add` against a hand-edited registration without checking for unknown/extra fields first | Silently destroys operator-authored config (comments, provider-specific keys) with no way to recover it, since neither CLI reads-merges-writes | Detect via the runtime's own structured read verb where one exists (Codex's `--json`); skip the write and report preserved-by-non-action rather than overwrite (Pitfall 10) |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|--------------|-------------------|
| `--apply` forces an unprompted Claude Code OAuth re-login for a registration the operator considered unchanged | Erodes trust in "idempotent re-run" exactly the way the existing docs-site guide already promises it won't ("converges ... without duplicate entries") | Distinguish cosmetic drift (whitespace/ordering) from substantive drift (URL/auth/header change) before deciding claude-code's remove-then-add sequence runs at all; state the re-login consequence explicitly when it will (Pitfall 6) |
| A scripted/CI `--apply` invocation discovers `claude plugin install`'s `-y` requirement only via a hung/failing non-interactive run | Confusing failure with no clear remediation, especially since none of engram's OWN existing write actions ever needed a confirmation-bypass flag before | Document and surface the plugin-consent flag/flow explicitly in `--help` and the docs-site guide the moment plugin-first delivery ships, not as a later doc pass |
| A machine with a stale plain-installed skill directory (from a pre-plugin-first `engram` binary) gets a silent, unexplained duplicate skill listing after upgrading and re-running `--apply` | Operator has no idea why their agent suddenly shows `curating-memory` twice, or which copy is "current" | Report the stale copy explicitly in the setup output the FIRST time plugin-first delivery detects it, rather than leaving the operator to notice the duplicate independently (Pitfall 9) |
| Codex operators with a hand-edited `config.toml` MCP entry get either a silent full-overwrite or a silent full-skip with no visible reasoning for which happened | Feels arbitrary — "sometimes engram touches my Codex config, sometimes it doesn't" — without the CLI ever explaining Codex's binary skip-or-clobber limitation | State plainly, in the reported outcome, that Codex offers no partial-preserve option and which of the two behaviors applied and why (Pitfall 10) |

## "Looks Done But Isn't" Checklist

- [ ] **`--apply` non-destructive guarantee:** Often verified only against a
  CLEAN fixture (no prior registration) — verify against a fixture with an
  EXISTING, unreproducible (custom-header) registration and confirm
  `--apply` performs ZERO write actions for that runtime, not just that
  preview reports it correctly (Pitfall 1).
- [ ] **Custom-header Codex handling:** Often verified only for the literal
  `Authorization: Bearer <ref>` shape — verify with a non-`Authorization`
  header name and confirm Codex routes to unsupported/preserve, never to a
  coerced `--bearer-token-env-var` call (Pitfall 2).
- [ ] **Drift-detection secret safety:** Often verified only for the
  already-proven bare-`${VAR}`-reference shape — verify what each read verb
  ACTUALLY prints for a header value that is NOT a bare reference, live,
  before trusting the comparison/rendering code (Pitfall 4).
- [ ] **Plugin-vs-plain skill de-duplication:** Often tested only against a
  clean machine — verify against a fixture pre-seeded with a PRE-existing
  plain-installed skill directory, confirming plugin-first delivery neither
  writes a second copy nor silently deletes the stale one (Pitfall 9).
- [ ] **Manpage generation determinism:** Often verified by "it produced a
  man page" — verify two consecutive generation runs are byte-identical
  (`DisableAutoGenTag` actually set), and that the new command is excluded
  from `help.golden`/the operator catalog the same way `completion` is
  (Pitfalls 11, 12).
- [ ] **`#560` timeout classification:** Often "fixed" by inspection of the
  diff alone — verify a fixture that forces a real context-deadline kill
  and asserts `Result.Reason` is DISTINGUISHABLE from an ordinary nonzero
  exit (Pitfall 13).

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|----------------|------------------|
| A custom-header Codex registration got silently coerced to `--bearer-token-env-var` (Pitfall 2), breaking auth against a gateway expecting a different header name | LOW–MEDIUM, per-user | `codex mcp remove engram` then manually re-register via a shell script or the gateway's own documented Codex integration path, since `codex mcp add` cannot express the header directly |
| An operator's Codex hand-edit was silently overwritten by `mcp add` (Pitfall 10) | HIGH if the hand-edit is not otherwise recorded | No engram-side recovery exists — `mcp add` never preserved the prior entry; restore from the operator's own backup/dotfiles history if one exists, which is exactly why this milestone should route to skip-not-overwrite once detected |
| Duplicate `curating-memory` (plain) and `engram:curating-memory` (plugin) both present (Pitfall 9) | LOW, per-user | Manually remove the reported stale plain directory (`~/.claude/skills/curating-memory`) once `engram setup` names it; never delete automatically |
| Claude Code forced an unwanted OAuth re-login during a reconcile-triggered rewrite (Pitfall 6) | LOW, per-user, but disruptive | Re-run `/mcp` → select `engram` → re-authenticate, exactly as the existing docs-site flow already documents for a fresh registration |
| A repeat of the 2026-09-10-shaped incident despite drift detection shipping (Pitfall 1) | HIGH — same recovery as the original incident | Restore the real registration manually per-runtime (`claude mcp add`/`codex mcp add`/`opencode mcp add` with the operator's own known-correct values); there is no automated undo, which is precisely why Pitfall 1's apply-time gate must exist before this milestone is considered done |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|--------------------|----------------|
| 1. `--apply` has no opt-out / drift detection doesn't gate writes | Drift-detection/reconcile phase | Fixture: `--apply` against a pre-seeded unreproducible registration issues ZERO write actions, not just a correct preview label |
| 2. Custom headers structurally inexpressible on Codex | Custom-headers phase, Codex sub-task | Fixture with a non-`Authorization` header name routes to unsupported/preserve, never `--bearer-token-env-var` |
| 3. Per-runtime header-syntax divergence | Custom-headers phase, per-runtime sub-tasks | Fixture asserting the literal separator character (`": "` vs `"="`) in each runtime's rendered `Action.Args` |
| 4. Read-probes may echo secret values for non-bare-reference headers | Drift-detection/reconcile phase | Live, read-only verification of each read verb's echo behavior (task, before comparison code lands); bound/sanitize any captured header text |
| 5. "Preserve" vs "converge on re-run" collide on claude-code's destructive remove-then-add | Drift-detection/reconcile phase, claude-code sub-task | Fixture: an unreproducible existing registration never reaches `claudeCodeRemoveAction` |
| 6. Reconcile forces unnecessary Claude Code OAuth re-login | Drift-detection/reconcile phase, claude-code sub-task | Fixture: cosmetic-only drift (whitespace/ordering) does not trigger the remove-then-add rewrite |
| 7. Lossy/non-deterministic read verbs undermine structured comparison | Drift-detection/reconcile phase | Per-runtime comparison-granularity decision recorded; fixture with an unrelated second MCP server proves opencode comparison isn't polluted by it |
| 8. Plugin install/marketplace-add without adequate consent | Plugin-first-delivery phase | Preview shows the marketplace source and declared install command before `--apply` ever passes `-y`; fixture reproduces the live "marketplace present, plugin absent" partial state found this session |
| 9. Plugin vs plain-install double registration / unsafe cleanup | Plugin-first-delivery phase | Fixture with a pre-existing plain skill directory: plugin-first delivery reports (never deletes) it; `SkillFormatNone` routing confirmed for plugin-managed runtimes |
| 10. Codex hand-edits clobbered by `mcp add`'s silent overwrite | Drift-detection/reconcile phase, Codex sub-task | Fixture with an unrecognized extra TOML key: `--apply` either skips Codex's write entirely or documents the overwrite, never a silent partial merge that doesn't exist |
| 11. Manpage generation regresses the completions design / follows stale PROJECT.md phrasing | Completions/manpages phase | `.goreleaser.yaml` re-verified directly (not from PROJECT.md prose) as the first task; `generate_completions_from_executable` absent from the diff; `DisableAutoGenTag` set |
| 12. New hidden man-generation command pollutes surface/catalog tests | Completions/manpages phase | `go test ./cmd/engram/... -run Golden` green with the new command present and `Hidden: true` |
| 13. `#560` `ctx.Err()` fix ships after, not before, plugin-install's timeout-sensitive tests are written | Sequenced before or alongside plugin-first-delivery phase | Fixture forcing a real context-deadline kill asserts a distinguishable `Result.Reason`, written against the FIXED `osRun` contract |

## Sources

**First-party (HIGH confidence — direct reads of this repo's own shipped
code and tests, this session):**
- `/Volumes/Code/github.com/seanb4t/engram/.planning/PROJECT.md` (Current
  Milestone: 2026-09-13.01 Setup v2 section)
- `/Volumes/Code/github.com/seanb4t/engram/.planning/research/PITFALLS.md`
  (prior milestone, 2026-08-23.01 — read first per the required-reading
  instruction; superseded pitfalls noted above, carried-forward ones cited
  by number)
- `internal/setup/claudecode.go`, `codex.go`, `opencode.go`, `generic.go`,
  `apply.go`, `plan.go`, `runtime.go`, `environment.go` (read directly)
- `internal/skills/install.go`, `agentsmd.go` (read directly)
- `docs-site/src/content/docs/guides/agent-setup.md` (read directly)
- `.goreleaser.yaml` (postflight hook, lines ~140–211, read directly)
- `cmd/engram/releaseconfig_test.go` (`TestReleaseConfigCaskInstallGate`,
  read directly)
- `cmd/engram/cmdwalk.go` (`isSkipped` doc-comment excerpt, read via grep
  context)
- `go.mod` (cobra/doc indirect-dependency comment, read directly)
- Live filesystem/JSON inspection of this machine's own
  `~/.claude/plugins/{installed_plugins.json,marketplaces/,cache/}` and
  `~/.claude/skills/` (read-only; no mutation) — the "marketplace present,
  plugin not installed, no plain skill copy" state cited in Pitfall 8 is
  this machine's REAL, current state, not a constructed fixture.

**First-party, live CLI `--help` output (HIGH confidence, read-only,
captured this session — no mutating command run, per the STRICT quality
gate):**
- `claude --version` (2.1.270), `claude plugin --help`, `claude plugin
  marketplace --help`, `claude plugin install --help`, `claude plugin
  update --help`, `claude mcp add --help`, `claude mcp get --help`
- `codex --version` (codex-cli 0.154.0), `codex plugin --help`, `codex
  plugin marketplace --help`, `codex plugin add --help`, `codex mcp add
  --help`, `codex mcp get --help`

**Not independently re-verified this session (MEDIUM confidence, carried
from `opencode.go`'s own code comments, themselves live-verified in the
PRIOR milestone against opencode 1.18.20):**
- `opencode mcp list`'s table shape, timing, and completeness limitations
- `opencode --header KEY=VALUE` syntax and the historical colon-space
  regression
- opencode's `{env:VAR}` substitution reliability
  (anomalyco/opencode#5299) — this session did not attempt to re-check that
  issue's current status; treat as still-open per the last recorded check

---
*Pitfalls research for: engram Setup v2 (plugin-first delivery, custom auth
headers, drift detection/reconcile, shell completions + manpages, #560)*
*Researched: 2026-09-13*
