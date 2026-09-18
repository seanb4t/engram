---
phase: "02"
slug: "custom-auth-headers"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-17"
---

# Phase 02 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| operator shell → `engram setup` argv / `ENGRAM_HEADERS` | Untrusted, operator-typed `NAME=ENVVAR` specs; the place a pasted literal secret would arrive. | Header specs (names only by contract) |
| `engram setup` → stderr / logs | Usage-error text is printed to the operator's terminal and may be captured by CI logs. | Error text |
| `engram setup` → `--output json` / text view | Row fields rendered by `viewRow`/`viewScalar`; a non-scalar field bypasses sanitization. | Result rows |
| CLI caller → `internal/setup.Options` | Validated specs cross into the leaf package; `EnvVar` is an opaque NAME the package never resolves. | Header NAME + ENVVAR name |
| `internal/setup` → runtime CLI argv | `Plan()` authors `Args` that `apply.go` execs directly (no shell); every header element is a reference string the runtime resolves at its own connect time. | argv reference strings (`${VAR}` / `{env:VAR}`) |
| `internal/setup` → pasted generic config | `Plan.Config` is a JSON document an operator pastes into an arbitrary third-party client. | JSON config with `${VAR}` references |
| generated tables / docs → agent or operator | Agents copy commands from `/engram-setup` tables; operators paste doc examples verbatim. | Command text, examples |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-02-01 | Information Disclosure | caller pastes a literal secret as `--header x-key=sk-live-…` | high | mitigate | `setupHeaderEnvVarRe` (POSIX identifier) in `cmd/engram/setup.go` rejects any RHS containing `$`, `{`, whitespace, `:`, `-`, or a leading digit; `TestSetupHeaderRejectsMalformedEnvVar` covers eight shapes in both lanes with zero effects. Residual (bare-identifier secret) — see AR-02-02 | closed |
| T-02-11 | Information Disclosure | rejected `--header` RHS echoed into the usage error (stderr, CI logs) | high | mitigate | `setupParseHeaders` formats only the NAME into every message; `TestSetupParseHeaders` / `TestSetupHeaderRejectsMalformedEnvVar` feed `sk-live-RHS-SENTINEL-3a9f` (8 occurrences in `setup_test.go`) and assert absence from `err.Error()` and stderr | closed |
| T-02-02 | Tampering | `--header Authorization=…` creating a second owner of the auth header | medium | mitigate | `strings.EqualFold(name, "Authorization")` is the first check in `setupParseHeaders` (also guarded in `claudecode.go` / `generic.go`); `TestSetupHeaderRejectsAuthorizationCollision` covers three letter cases in both lanes | closed |
| T-02-03 | Information Disclosure | `internal/setup` or the CLI resolving a header's env var (VALUE on argv, Config, preview, JSON, logs) | high | mitigate | `internal/setup/*.go` (non-test) has one `os.Getenv` — the `OSEnvironment` seam in `environment.go`; runtime files interpolate `EnvVar` as a NAME inside `${…}` / `{env:…}`; the CLI's only new read is `ENGRAM_HEADERS` (a NAME list); `TestNoSecretInArgs` (`plan_test.go`, `generic_test.go`) asserts the sentinel absent from every Args element and Config for every runtime × mode; `TestSetupHeaderValidWithEveryAuthMode`'s fake env fails on any read other than `XDG_CONFIG_HOME` | closed |
| T-02-04 | Tampering | codex silently downgrading/coercing a header (writing config.toml, using an override flag, or registering without the header while reporting success) — incl. docs implying otherwise | high | mitigate | `codexRuntime.Plan()` (`internal/setup/codex.go:93`) returns `Plan{}` + `ErrHeaderUnsupported` as its first statement when `opts.Headers` is non-empty; `codex.go` has zero `os.WriteFile`/`os.Create`; `TestCodexDeclinesHeaders` asserts `HomeDir` never reached; `TestSetupGeneratedInvocations` asserts codex's row is `failed` with `command == ""`; `/engram-setup` prose states the decline and remedy. (The `http_headers` struct tags in `codex.go` are Phase 4's read-only `codex mcp get --json` parse, not a writer.) | closed |
| T-02-07 | Repudiation | operator cannot tell a header gap from an auth-mode gap | low | mitigate | Distinct `ErrHeaderUnsupported` vs `ErrAuthModeUnsupported` sentinels; `TestCodexDeclinesHeaders` asserts `errors.Is` holds for the former and not the latter; reason names gap + remedy | closed |
| T-02-05 | Tampering | argv injection via a header NAME/ENVVAR containing shell metacharacters | low | mitigate | `apply.go` execs `Args` with no shell; display goes through `quoteArgs` / `safeArgRunes` (`internal/setup/quote.go`); RFC 7230 token / POSIX identifier grammar rejects whitespace, `;`, quotes, `$`, `{` at the CLI boundary | closed |
| T-02-08 | Tampering | test suite invoking a real third-party CLI or the operator's `$HOME` | medium | mitigate | Every CLI test uses `withFakeSetupEnv` (70 uses) + `fakeSkillsEnv`; package tests drive `Plan()` against a fake `Environment`; `TestGenericStartsNoProcess` fails on any `LookPath`/`Run`; `TestSetupHeaderCodexDeclined` records `Run` calls and asserts zero `add` invocations | closed |
| T-02-09 | Tampering | shared cross-runtime formatter re-introducing the opencode colon-space regression (silent registration failure) | medium | mitigate | `openCodeHeaderArgs` lives in `opencode.go` and authors `NAME={env:VAR}` (7 `{env:` hits; zero `": "` dialect hits); `TestOpenCodeHeaders` pins exact elements | closed |
| T-02-10 | Information Disclosure | generic's ordered `MarshalJSON` hand-rolling escaping and emitting unescaped `<`, `>`, `&`, quote into a pasted config | medium | mitigate | Every key/value passes through `json.Marshal` (7 uses in `generic.go`); `TestGenericHeaders` asserts the `--token-file` literal keeps its `<…>` escaping byte-for-byte | closed |
| T-02-06 | Information Disclosure | nested (array/map) `headers` row field bypassing `viewScalar` sanitization | medium | mitigate | `setupRuntimeRow.Headers` is `string` (`setup.go:511`); fixtures run under `TestOperatorViewFixturesHaveNoUnsanitizedNesting`; `TestSetupHeaderOrderIndependent` asserts the raw JSON type is `string` | closed |
| T-02-12 | Tampering | `ENGRAM_HEADERS` in a contributor's shell baking into committed help/catalog goldens | low | mitigate | `envDerivedFlagDefaults["setup"] = {"runtime": true, "header": true}` (`golden_test.go:70`) blanks the flag's `DefValue` during golden generation | closed |
| T-02-13 | Tampering | hand-edited or stale generated tables diverging from the real `ClaudeCode.Plan` | medium | mitigate | `surfacesgen --check-setup` (`internal/surfacesgen/main.go`) and `TestRenderRealPlans` (`internal/setupgen/setupgen_test.go`) compare committed bytes against `Render()` | closed |
| T-02-14 | Information Disclosure | doc or table example carrying a literal secret beside a header name | medium | mitigate | Examples use env var NAMEs (`LITELLM_KEY`, `ENGRAM_TOKEN`); grep of `docs-site/content` + `skill` for `sk-…` literals finds none (the one `Bearer <token>` hit in `engram-setup.md:38` is a placeholder in auth-mode prose, not a value); `Render()`'s synthetic env fails on any environment read | closed |
| T-02-SC | Tampering | npm/pip/cargo installs | low | accept | Not applicable — zero new packages; `internal/setup` stays stdlib-only (`TestSetupPackageIsStdlibOnlyLeaf`); `go.mod`/`go.sum` byte-unchanged in every plan — see AR-02-01 | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-02-01 | T-02-SC | No package-manager install task in any of the four plans; `internal/setup` remains a stdlib-only leaf; `go.mod`/`go.sum` byte-unchanged. Supply-chain control is not applicable. | Plan author (02-01..04 threat models) | 2026-09-17 |
| AR-02-02 | T-02-01 (residual) | A secret that happens to be a bare POSIX identifier is indistinguishable from a variable name. Help text documents "ENVVAR is the NAME of an environment variable"; if pasted, the runtime resolves a nonexistent variable — engram never places the value anywhere. | Plan author (02-03 threat model) | 2026-09-17 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-17 | 15 | 15 | 0 | /gsd-secure-phase (orchestrator, L1 grep-depth short-circuit — register authored at plan time, ASVS L1) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-17
