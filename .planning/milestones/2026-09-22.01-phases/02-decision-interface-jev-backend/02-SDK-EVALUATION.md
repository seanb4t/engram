sdk_module: github.com/OpenRouterTeam/go-sdk
sdk_version: v0.8.19
legitimacy: approved
verdict: ADOPT-AND-WRAP-CANDIDATE
resolution: reject-hand-write

# DEC-05 SDK Evaluation — OpenRouter Go SDK

This is the D-07 evaluation doc: a phase doc committed before any client code
(`git ls-files internal/decide` is empty as of this commit). Plan 02-01 (this
plan) records D-05(a) and the static half of D-05(b) plus the package-legitimacy
facts, all from live command output captured during execution — never copied
from `02-RESEARCH.md`, since this SDK ships several releases a day. Plan 02-02
completes the behavioral half of D-05(b), the D-06 verdict and the durable
decision record.

## Candidate

- Module path: `github.com/OpenRouterTeam/go-sdk`
- Version: `v0.8.19`
- Publish time: `2026-09-22T21:41:17Z`
- GoVersion (module's own toolchain requirement): `1.25.10` — engram is `go 1.26.7`; compatible (engram's toolchain is newer than the SDK's floor).
- License: `Apache-2.0`
- Repository: `https://github.com/OpenRouterTeam/go-sdk`

## D-05(a) — maintained upstream, current module path

Facts quoted verbatim from command output captured this session (all commands run from the
session scratchpad, outside engram's module, per the executor notes):

`go list -m -json github.com/OpenRouterTeam/go-sdk@latest`:

```json
{
	"Path": "github.com/OpenRouterTeam/go-sdk",
	"Version": "v0.8.19",
	"Query": "latest",
	"Time": "2026-09-22T21:41:17Z",
	"Dir": "/Users/sean/go/pkg/mod/github.com/!open!router!team/go-sdk@v0.8.19",
	"GoMod": "/Users/sean/go/pkg/mod/cache/download/github.com/!open!router!team/go-sdk/@v/v0.8.19.mod",
	"GoVersion": "1.25.10"
}
```

No `Deprecated` field and no `Retracted` field are present in this output — their absence is
the module proxy's positive signal that the module is neither deprecated nor retracted.

`go list -m -versions github.com/OpenRouterTeam/go-sdk`: 237 published versions total (counted
by `tr ' ' '\n' | grep -c '^v'` over the raw output). Last five: `v0.8.15 v0.8.16 v0.8.17
v0.8.18 v0.8.19`.

`go mod download -json github.com/OpenRouterTeam/go-sdk@v0.8.19` (places the module source in
the local module cache; does not compile or run it):

```json
{
	"Path": "github.com/OpenRouterTeam/go-sdk",
	"Version": "v0.8.19",
	"Dir": "/Users/sean/go/pkg/mod/github.com/!open!router!team/go-sdk@v0.8.19",
	"Sum": "h1:2aX+H/hDRPZrgN3R+nUYNfe/GKcS0bdlwp/NVFnoMIY=",
	"GoModSum": "h1:8vFtUZ9I2YiDntYbck42wQhrx9DsJN5PHbViaHwTkXc=",
	"Origin": {
		"VCS": "git",
		"URL": "https://github.com/OpenRouterTeam/go-sdk",
		"Hash": "6e15ae961302f38db2a153293010e98a19cf2160",
		"Ref": "refs/tags/v0.8.19"
	}
}
```

`gh api repos/OpenRouterTeam/go-sdk --jq '{archived, pushed_at, license: .license.spdx_id,
default_branch, html_url}'`:

```json
{"archived":false,"pushed_at":"2026-09-22T21:43:52Z","license":"Apache-2.0","default_branch":"main","html_url":"https://github.com/OpenRouterTeam/go-sdk"}
```

**Result: PASS.** Not deprecated, not retracted, repository not archived, and its most recent
release (`v0.8.19`, `2026-09-22T21:41:17Z`) is 1 day before this evaluation (2026-09-23) — well
inside the 90-day maintenance window.

## Surface inventory (static, no SDK code executed)

Every file below was read from the module cache (`go mod download`'s `Dir`), never compiled or
run. Source citations are `<file>:<line-range or symbol>` relative to the module root.

### Request types

- `DecisionsRequest` (`models/components/decisionsrequest.go`): `Model string`,
  `Provider optionalnullable.OptionalNullable[ProviderPreferences]`,
  `Questions map[string]Questions`, `SessionID *string`, `State State`, `Trace *TraceConfig`,
  and **`User *string`** — a field not named anywhere in `COVERAGE.md`, `02-RESEARCH.md`,
  `02-CONTEXT.md` or the spike findings (see Coverage reconciliation below).
- `Questions` (same file): discriminated union of `DecisionsNoulQuestion` |
  `DecisionsChoiceQuestion` | `DecisionsScoreQuestion`, keyed by a `Type` string discriminator
  (`"noul"`/`"choice"`/`"score"`), with `CreateQuestionsNoul`/`CreateQuestionsChoice`/
  `CreateQuestionsScore` constructors and custom `UnmarshalJSON`/`MarshalJSON`.
- `State` (same file): discriminated union of `Str *string` | `MapOfAny map[string]any` |
  `ArrayOfAny []any`, with the same constructor/marshal shape.
- `DecisionsNoulQuestion` (`models/components/decisionsnoulquestion.go`): `Criteria
  *DecisionsNoulQuestionCriteria{True, False}` (each of `True`/`False` its own `str`/`mapOfAny`/
  `arrayOfAny` union), `Instructions` (its own `str`/`mapOfAny`/`arrayOfAny` union), `Type
  "noul"`.
- `DecisionsChoiceQuestion` (`models/components/decisionschoicequestion.go`): `Criteria
  map[string]*Criteria` (each `Criteria` value a `str`/`mapOfAny`/`arrayOfAny` union),
  `Instructions` (its own `str`/`mapOfAny`/`arrayOfAny` union), `Type "choice"`.
- `DecisionsScoreQuestion` (`models/components/decisionsscorequestion.go`): `Criteria
  []Criterion` — an **ordered array**, matching the spike's "score questions take criteria as
  an ordered array" finding — each `Criterion` its own `str`/`mapOfAny`/`arrayOfAny` union,
  `Instructions` (its own `str`/`mapOfAny`/`arrayOfAny` union), `Type "score"`.

### Answer and usage types

- `DecisionsResponse` (`models/components/decisionsresponse.go`): `Answers map[string]Answers`,
  `ID *string`, `Model string`, `Provider *string`, `Usage DecisionsResponseUsage`.
- `DecisionsResponseUsage` (same file): `Cost *float64`, `InputTokens int64`, `OutputTokens
  int64`.
- `Answers` (same file): discriminated union of `DecisionsNoulAnswer` | `DecisionsChoiceAnswer`
  | `DecisionsScoreAnswer`, keyed by `Type`, plus an `UnknownRaw json.RawMessage` fallback
  (`Type == "UNKNOWN"`) — `UnmarshalJSON` sets this whenever the discriminator is missing,
  unparseable, or not one of `noul`/`choice`/`score`; `GetUnknownRaw()`/`IsUnknown()` accessors
  are exported.
- `DecisionsNoulAnswer` (`models/components/decisionsnoulanswer.go`): `Noul float64`, `Type
  "noul"`.
- `DecisionsChoiceAnswer` (`models/components/decisionschoiceanswer.go`): `Choice string`,
  `Confidence *float64`, `Probabilities map[string]float64`, `Type "choice"`.
- `DecisionsScoreAnswer` (`models/components/decisionsscoreanswer.go`): `Confidence *float64`,
  `Legend map[string]Legend` (each `Legend` value its own `str`/`mapOfAny`/`arrayOfAny` union),
  `Probabilities map[string]float64`, `Score float64`, `Type "score"`.

### Operation mechanics

- `Decisions.Create`'s default server is `operations.CreateAPIAlphaDecisionsServerList[0]` =
  `"https://openrouter.ai"` (`models/operations/createapialphadecisions.go`). It joins
  `url.JoinPath(baseURL, "alpha/decisions")` via `"/api/alpha/decisions"` literally
  (`decisions.go:55`), giving **`https://openrouter.ai/api/alpha/decisions`** by default. Only
  the per-call `operations.WithServerURL` option (`o.ServerURL`) replaces `baseURL`
  (`decisions.go:51-53`); the client-level `openrouter.WithServerURL`
  (`sdkConfiguration.ServerURL`) is never read by `Decisions.Create` — confirmed by reading the
  full `baseURL` resolution, which starts from `CreateAPIAlphaDecisionsServerList[0]` and checks
  only `o.ServerURL`, never `s.sdkConfiguration.ServerURL` (`decisions.go:50-53`).
- `SystemOne.Create` (the sibling operation sharing the same request/response types,
  `systemone.go`) resolves its base URL differently: when `o.ServerURL` is unset it calls
  `s.sdkConfiguration.GetServerDetails()` (`systemone.go:50-55`), which reads the SDK's normal
  `ServerList[ServerProduction]` = `"https://openrouter.ai/api/v1"` (`openrouter.go:26-28`), then
  joins `"/systemone"` — `https://openrouter.ai/api/v1/systemone` by default, matching
  `COVERAGE.md`'s existing `systemone.create` row.
- Default retry policy, used whenever neither a per-call `operations.WithRetries` nor an
  SDK-level `RetryConfig` is set (`decisions.go:103-119`): `Strategy: "backoff"`,
  `InitialInterval: 500` (ms), `MaxInterval: 60000` (ms, 60s), `Exponent: 1.5`,
  `MaxElapsedTime: 3600000` (ms, 1h), `RetryConnectionErrors: true`. The retry loop's
  `StatusCodes` list is `["5XX"]` only (`decisions.go:125-127`) — 429 is **not** retried by this
  default policy, only 5xx responses and connection errors.
- Response body reading: every status branch calls `utils.ConsumeRawBody(httpRes)`
  (`internal/utils/utils.go:375-386`), which does `defer res.Body.Close(); rawBody, err :=
  io.ReadAll(res.Body)` with **no size bound**, then rewraps `res.Body` as
  `io.NopCloser(bytes.NewBuffer(rawBody))` so a caller-installed `http.RoundTripper` never sees
  the body a second time.
- Non-2xx decode: each named status (400/401/402/403/404/413/429/500/502/503/524/529) decodes
  the raw JSON body into its own typed `sdkerrors.*ResponseError` via
  `utils.UnmarshalJsonFromResponseBody` (`internal/utils/utils.go:37-47`, itself `io.ReadAll` +
  `UnmarshalJSON`); any other 4xx/5xx status, a non-JSON content type, or a JSON decode failure
  instead returns `sdkerrors.NewAPIError(message, statusCode, body, httpRes)` — the decode error
  itself propagates as a wrapped Go error (`"error unmarshaling json response body: %w"`), it is
  never silently swallowed (`decisions.go:200-495`).
- The `code` field type: every per-status `*ResponseErrorData` (e.g.
  `BadRequestResponseErrorData`, `models/components/badrequestresponseerrordata.go:11`) types
  its `Code` field `int64`. A JSON string `code` (as the LiteLLM pass-through sends per the
  spike findings) fails `json.Unmarshal` into that field — this is exactly RESEARCH's Pitfall 1
  / Open Question 1, still untested as of this static read; plan 02-02's behavioral evaluation
  observes it live.

### Escape hatches

Request-side (all already `OPT-OUT`'d by `COVERAGE.md`'s `request.state non-object forms` and
`question.instructions and criteria as structured` rows):

- `State.MapOfAny map[string]any` / `State.ArrayOfAny []any` (`decisionsrequest.go`)
- `Criteria.MapOfAny` / `Criteria.ArrayOfAny` — choice question's per-option guidance
  (`decisionschoicequestion.go`)
- `DecisionsChoiceQuestionInstructions.MapOfAny` / `.ArrayOfAny` (`decisionschoicequestion.go`)
- `True.MapOfAny` / `.ArrayOfAny`, `False.MapOfAny` / `.ArrayOfAny` — noul question criteria
  (`decisionsnoulquestion.go`)
- `DecisionsNoulQuestionInstructions.MapOfAny` / `.ArrayOfAny` (`decisionsnoulquestion.go`)
- `Criterion.MapOfAny` / `.ArrayOfAny` — score question's ordered criteria array items
  (`decisionsscorequestion.go`)
- `DecisionsScoreQuestionInstructions.MapOfAny` / `.ArrayOfAny` (`decisionsscorequestion.go`)

Answer-side:

- `Legend.MapOfAny map[string]any` / `Legend.ArrayOfAny []any` — `DecisionsScoreAnswer.Legend
  map[string]Legend` (`decisionsscoreanswer.go`). Already covered generically by `COVERAGE.md`'s
  `answer.score` `INTEGRATE` row (legend is part of the score-answer shape); no separate row
  needed.
- `Answers.UnknownRaw json.RawMessage` — not itself typed `map[string]any`/`[]any`, but the same
  kind of escape hatch for a genuinely unrecognized answer discriminator. Already covered
  generically by the three `answer.*` `INTEGRATE` rows (the union only falls back here when none
  of noul/choice/score match).

## Dependency footprint

| Module | Required by go-sdk's `go.mod` | Already in engram's `go.sum`? |
|---|---|---|
| `github.com/spyzhov/ajson v0.8.0` | direct | **No** — `rg -o -F 'github.com/spyzhov/ajson ' go.sum \| wc -l` from the repo root prints `0`. The one genuinely new dependency if the SDK is adopted. |
| `github.com/stretchr/testify v1.12.1` | direct | Yes — `2` hits. |
| `go.yaml.in/yaml/v3 v3.0.5` | indirect | Yes — `3` hits. |

## Coverage reconciliation

- Every capability inventoried above already has a `COVERAGE.md` row, with **one exception**:
  `DecisionsRequest.User *string` (`models/components/decisionsrequest.go`) — an optional
  end-user identifier the request body accepts. No existing `COVERAGE.md` row, `02-RESEARCH.md`
  section, `02-CONTEXT.md` decision, or spike finding names it. Added as a new row,
  `request.user`, `OPT-OUT` (request-side, no planned consumer) — see the reconciled matrix.
- `SystemOne.Create`'s default base-URL resolution
  (`sdkConfiguration.GetServerDetails()`/`ServerList[ServerProduction]` =
  `https://openrouter.ai/api/v1`, joined with `/systemone`) differs mechanically from
  `Decisions.Create`'s (`CreateAPIAlphaDecisionsServerList[0]` = `https://openrouter.ai`, joined
  with `/api/alpha/decisions`) — both resolve to the exact paths `COVERAGE.md` already records
  (`/api/v1/systemone` and `/api/alpha/decisions` respectively). No row change needed; noted here
  for the record since this plan re-derived it from source rather than trusting the prior static
  read.
- No capability row is dropped relative to the `<interfaces>` block's v0.8.19 leads — the
  version executed this session (`v0.8.19`) is the identical version those leads were drawn
  from, so nothing needed re-verifying against a version drift.

## Package legitimacy

Facts a human needs to approve or reject the Task 2 checkpoint (see that checkpoint for the
verification steps):

- Candidate: `github.com/OpenRouterTeam/go-sdk` @ `v0.8.19`, published `2026-09-22T21:41:17Z`.
- Org match: OpenRouter's own docs (`https://openrouter.ai/docs/client-sdks/go`) link to this
  exact repository — the human verifies this live at the checkpoint.
- License: `Apache-2.0`. Repository not archived, default branch `main`, 237 published
  versions (last five: `v0.8.15 v0.8.16 v0.8.17 v0.8.18 v0.8.19`), last push
  `2026-09-22T21:43:52Z`.
- Transitive: `github.com/spyzhov/ajson v0.8.0` — the one genuinely new dependency this
  adoption would add; not independently audited here (inherited, never imported directly by
  engram code) — the human confirms it is an established JSON library, not a look-alike.

**Outcome: approved (2026-09-23).** The user approved plan 02-02 downloading, compiling
and running `github.com/OpenRouterTeam/go-sdk` pinned to exactly `v0.8.19` in the nested
module under `.planning/`. The orchestrator independently confirmed via the GitHub API:
the repository is not archived, licensed Apache-2.0, owned by `OpenRouterTeam`, and
`v0.8.19` was published `2026-09-22T21:42:50Z` (release) with last push
`2026-09-22T21:43:52Z`. `spyzhov/ajson` is MIT-licensed, created 2019-03-07, has 292
stars, and is not archived.

## Behavioral evaluation (plan 02-02)

Harness: `.planning/phases/02-decision-interface-jev-backend/sdk-eval/` (nested module
`engram.invalid/sdkeval`, pinned to `github.com/OpenRouterTeam/go-sdk v0.8.19`). Run with
`go -C .planning/phases/02-decision-interface-jev-backend/sdk-eval test -run
'^TestSDKEvaluation$' -count=1 -race -v ./...` (E01–E08) and the same with
`-run '^TestSDKConcurrency$'` (E09). Every observation line below is pasted verbatim from that
run. `t.Logf` reports a check's PASS/FAIL against the criterion stated in the plan; the harness
itself exited 0 both times (a subtest logging `FAIL` is a recorded finding, never a harness
failure — only a fixture that fails to parse or a server that fails to start would `t.Fatal`).

| check | question | observed | PASS/FAIL |
|---|---|---|---|
| E01 path fidelity | Does one documented per-call option (`operations.WithServerURL`, with or without a trailing `/api` trimmed) reach `/api/alpha/decisions` for an OpenRouter-shaped base AND `/openrouter/alpha/decisions` for a LiteLLM-pass-through-shaped base? | `EVAL-E01 FAIL B1(OpenRouter shape, base="http://127.0.0.1:60209/api", want=/api/alpha/decisions)=true via="http://127.0.0.1:60209" attempts=[WithServerURL("http://127.0.0.1:60209/api")->path="/api/api/alpha/decisions", WithServerURL("http://127.0.0.1:60209")->path="/api/alpha/decisions"]; B2(LiteLLM shape, base="http://127.0.0.1:60209/openrouter", want=/openrouter/alpha/decisions)=false via="" attempts=[WithServerURL("http://127.0.0.1:60209/openrouter")->path="/openrouter/api/alpha/decisions"]` | FAIL |
| E02 request fidelity | Decoding the captured request body: are top-level keys exactly `model`/`state`/`questions`, is noul criteria an object with `true`/`false`, is choice criteria an object, is score criteria an array, are instructions strings? | `EVAL-E02 PASS top_level_keys_extra=[] noul_criteria_has_true=true noul_criteria_has_false=true noul_instructions_kind=string choice_criteria_kind=object choice_instructions_kind=string score_criteria_kind=array score_instructions_kind=string` | PASS |
| E03 typed decode (D-05 bar b) | Are every answer value, usage, model, id and provider reachable through typed fields (no `map[string]any`/`[]any`/`UnknownRaw`) across `fixtureHappy`, `fixtureBatch50` and `fixtureScore`? | `EVAL-E03 PASS happy/batch50/score answers, usage, model, id and provider all reachable through typed fields; no map[string]any/[]any/UnknownRaw on the answer/usage side` | PASS |
| E04 caller-supplied client | Does a caller-supplied `*http.Client`/`http.RoundTripper` (`openrouter.WithClient`) see every request `Create` makes? | `EVAL-E04 PASS calls_made=3 requests_seen_by_caller_supplied_client=3` | PASS |
| E05 error dialects (DEC-04, D-12) | For every error fixture (both OpenRouter numeric-`code` and LiteLLM string-`code` dialects, plus a non-JSON HTML 502), is the HTTP status derivable from the returned error without engram intercepting the response? | `EVAL-E05 FAIL OpenRouter400Choices(card256)(want=400): type=*sdkerrors.BadRequestResponseError api_error=false named_type=true derived_status=400 derivable=true; OpenRouter401(badkey)(want=401): type=*sdkerrors.UnauthorizedResponseError api_error=false named_type=true derived_status=401 derivable=true; OpenRouter400BadType(badtype)(want=400): type=*sdkerrors.BadRequestResponseError api_error=false named_type=true derived_status=400 derivable=true; OpenRouter400MaxTokens(oversize)(want=400): type=*sdkerrors.BadRequestResponseError api_error=false named_type=true derived_status=400 derivable=true; ChatPath400(chatpath)(want=400): type=*sdkerrors.BadRequestResponseError api_error=false named_type=true derived_status=400 derivable=true; LiteLLM401(string code)(want=401): type=*fmt.wrapError api_error=false named_type=false derived_status=0 derivable=false; LiteLLM403(string code)(want=403): type=*fmt.wrapError api_error=false named_type=false derived_status=0 derivable=false; OpenRouter402(want=402): type=*sdkerrors.PaymentRequiredResponseError api_error=false named_type=true derived_status=402 derivable=true; OpenRouter404(want=404): type=*sdkerrors.NotFoundResponseError api_error=false named_type=true derived_status=404 derivable=true; OpenRouter429(want=429): type=*sdkerrors.TooManyRequestsResponseError api_error=false named_type=true derived_status=429 derivable=true; OpenRouter500(want=500): type=*sdkerrors.InternalServerResponseError api_error=false named_type=true derived_status=500 derivable=true; OpenRouter502(want=502): type=*sdkerrors.BadGatewayResponseError api_error=false named_type=true derived_status=502 derivable=true; OpenRouter503(want=503): type=*sdkerrors.ServiceUnavailableResponseError api_error=false named_type=true derived_status=503 derivable=true; OpenRouter524(want=524): type=*sdkerrors.EdgeNetworkTimeoutResponseError api_error=false named_type=true derived_status=524 derivable=true; OpenRouter529(want=529): type=*sdkerrors.ProviderOverloadedResponseError api_error=false named_type=true derived_status=529 derivable=true; HTML502(non-JSON)(want=502): type=*sdkerrors.APIError api_error=true named_type=false derived_status=502 derivable=true` | FAIL |
| E06 errors.Is propagation (DEC-04) | (a) Does a RoundTripper transport error survive `errors.Is`? (b) Does a body-`Read` error after 64 bytes survive `errors.Is`? (c) How many bytes of a 5 MiB body does the SDK read? | `EVAL-E06 PASS a_transport_err_is_sentinel=true b_body_read_err_is_sentinel=true c_bytes_read=5242880/5242880 c_read_whole_body=true` | PASS (a, b); (c) unbounded read confirmed |
| E07 retries (D-11) | Request counts under a 3 s deadline: no retry config, `operations.WithRetries(retry.Config{Strategy:"none"})`, and a tightly-bounded backoff config. Can retries be turned off entirely? | `EVAL-E07 PASS no_retry_config_requests=3 retries_strategy_none_requests=1 bounded_backoff_attempt_requests=3` | PASS |
| E08 deadline | Does a 200 ms context deadline against a 2 s-sleeping server surface as `errors.Is(err, context.DeadlineExceeded)`? | `EVAL-E08 PASS err=error sending request: Post "http://127.0.0.1:60262/api/alpha/decisions": context deadline exceeded is_deadline_exceeded=true` | PASS |
| E09 concurrency (DEC-05 concurrency edge) | One SDK client driven from 4 goroutines × 25 calls under `-race` — do all 100 succeed with no race? | `EVAL-E09 PASS all 100 calls succeeded across 4 goroutines (-race clean)` | PASS |
| E10 footprint | Set difference between `go -C sdk-eval list -m all` and `go list -m all` (repo root)? | New modules: `github.com/OpenRouterTeam/go-sdk v0.8.19` (direct), `github.com/spyzhov/ajson v0.8.0` (indirect) — matches 02-01's Dependency footprint table exactly. Already shared with engram's `go.sum`: `github.com/stretchr/testify v1.12.1`, `go.yaml.in/yaml/v3 v3.0.5`. `git status --porcelain -- go.mod go.sum` (repo root) is empty; `git ls-files internal/decide` is empty. | PASS (matches static estimate, no surprise transitive) |

## Verdict (plan 02-02)

**`verdict: ADOPT-AND-WRAP-CANDIDATE`** — rule **R3** fired: D-05(a) passed (plan 02-01) and E03
passed (R1/R2 do not apply), but three independent R3 triggers are present —

- **E01 FAILED.** `Decisions.Create` always joins whatever base URL is in effect with the
  literal string `"/api/alpha/decisions"` (`decisions.go:55`); only the per-call
  `operations.WithServerURL` is honored (the client-level `openrouter.WithServerURL` is never
  read by this operation, confirmed both statically in 02-01 and live here). No documented
  option reaches `/openrouter/alpha/decisions` for the LiteLLM pass-through shape — the OpenRouter
  shape works (trim the trailing `/api`), but the gateway shape cannot be expressed without a
  URL-rewriting `http.RoundTripper`.
- **E05 FAILED.** For the LiteLLM dialect's string `"code":"401"`/`"403"`, the SDK's per-status
  typed error structs (`Code int64`) fail to unmarshal and `Create` returns a bare
  `*fmt.wrapError` carrying no HTTP status at all — neither `*sdkerrors.APIError` nor a named
  per-status type. Every OpenRouter-dialect fixture (numeric `code`), including the non-JSON
  HTML 502, classified correctly by type; only the LiteLLM string-`code` dialect breaks
  classification. This is exactly RESEARCH's Pitfall 1 / Open Question 1, now confirmed live.
- **E06(c) showed an unbounded read.** `utils.ConsumeRawBody` performs a plain `io.ReadAll` with
  no byte bound (confirmed: read all 5,242,880 bytes of the synthetic body) — DEC-04's
  response-byte-bound requirement is not met unaided.

E04, E07, E08, E09 and E06(a)/(b) all passed: a caller-supplied `*http.Client` sees every
request, SDK retries can be turned off entirely (`Strategy: "none"` → 1 request vs. 3 with no
config), a context deadline surfaces as `context.DeadlineExceeded`, and both transport-level and
body-Read errors stay `errors.Is`-matchable through the SDK's wrapping.

### Resolution

**2026-09-23 — `resolution: reject-hand-write`.** Verdict `ADOPT-AND-WRAP-CANDIDATE` (rule R3)
routes to the `blocking-human` checkpoint D-06 reserves for exactly this case — R3 is "stop and
ask," not a directive to adopt. The user chose `reject-hand-write` at that checkpoint. **This is
not an override of the recorded verdict or of D-06**: `reject-hand-write` is one of the two
in-table options the checkpoint offers when the verdict is `ADOPT-AND-WRAP-CANDIDATE`, and the
user selected it on the recorded evidence rather than overriding the rule table.

Reason: all three R3 triggers (E01, E05, E06(c)) require engram to own the fix regardless of
which branch is chosen — E01's hardcoded `/api/alpha/decisions` cannot reach the LiteLLM
pass-through without a URL-rewriting RoundTripper, E05's LiteLLM string `code` breaks the SDK's
typed error decode and must be reclassified by HTTP status anyway, and E06(c)'s unbounded
`ConsumeRawBody` needs a body-bounding wrapper regardless of which client owns the request. With
path rewriting, status classification, byte bounding and retry override all falling to engram
either way, adopting the SDK leaves only generated request/answer types as the benefit, at the
cost of a new direct module dependency (plus `spyzhov/ajson` transitively) on an alpha API that
ships several releases a day. `internal/decide/jev` is hand-written on `net/http` +
`encoding/json`, following the `internal/embed`/`internal/summarize` pattern.

## Hand-write recipe

`net/http` plus `encoding/json` wire structs following `internal/embed`/`internal/summarize`'s
`New(base, key, model string, opts ...Option)` shape: `httpdrain.Drain` on every response path
(request and error bodies alike), `otelhttp` transport via the same pattern those two packages
use for the `decide` span (D-13), an endpoint built as
`strings.TrimRight(base, "/") + "/alpha/decisions"` (works unmodified for both the OpenRouter
shape and the LiteLLM pass-through shape — no URL rewriting needed, unlike the wrap path), and
classification by the HTTP status code the standard library already exposes on `*http.Response`
(both the OpenRouter numeric-`code` and LiteLLM string-`code` error bodies collapse to the same
`ErrDecision*` sentinel per D-12, since classification never depends on decoding `error.code`).

## Durable decision record (plan 02-02)

```
scope: repo:engram
category: decision
tags: [dec-05, openrouter-go-sdk, decisions-api, jev]
summary: DEC-05 SDK evaluation of github.com/OpenRouterTeam/go-sdk@v0.8.19 against the Decisions API — verdict ADOPT-AND-WRAP-CANDIDATE (rule R3); D-06 checkpoint resolved reject-hand-write.
content: |
  Evaluated github.com/OpenRouterTeam/go-sdk@v0.8.19 (evaluation date 2026-09-23) against DEC-05's
  D-05(a)/(b) bar and D-06's DEC-03/DEC-04/DEC-06 fit. D-05(a) passed (not deprecated/retracted,
  repository not archived, most recent release one day before evaluation). D-05(b)'s behavioral
  half (E03) passed: every answer/usage/model/id/provider value is reachable through typed Go
  fields with no map[string]any/[]any/UnknownRaw escape hatch on the answer side. The fixed D-06
  rule table's R3 fired via three independent findings: (1) E01 — no documented per-call option
  reaches the LiteLLM pass-through path shape (/openrouter/alpha/decisions); Decisions.Create
  always joins the literal "/api/alpha/decisions" onto whatever base is configured, and the
  client-level WithServerURL is never read by this operation. (2) E05 — the LiteLLM dialect's
  string "code" field (vs. OpenRouter's numeric code) breaks the SDK's typed error decode
  entirely for 401/403 from that gateway, returning a bare wrapped error with no derivable HTTP
  status; every OpenRouter-dialect fixture classified correctly. (3) E06(c) — the SDK's raw body
  read is unbounded (read a full 5 MiB test body with no size cap). E04 (caller-supplied
  *http.Client), E07 (retries fully disable-able via retry.Config{Strategy:"none"}), E08 (context
  deadline surfaces as context.DeadlineExceeded) and E06(a/b) (errors.Is propagation through both
  transport and body-read errors) all passed. Full evidence, the wrap recipe and the hand-write
  recipe are recorded in
  .planning/phases/02-decision-interface-jev-backend/02-SDK-EVALUATION.md.

  Outcome: resolved reject-hand-write (2026-09-23). The user chose to hand-write
  internal/decide/jev on net/http + encoding/json following the internal/embed/internal/summarize
  pattern, at the blocking-human D-06 checkpoint ADOPT-AND-WRAP-CANDIDATE routes to — an in-table
  choice, not an override. Rationale: all three R3 triggers (E01 path, E05 LiteLLM string-code
  decode, E06(c) unbounded read) require engram to own path rewriting, status classification,
  byte bounding and retry override regardless of which branch is chosen, leaving only generated
  types as the SDK's benefit against a new direct dependency on an alpha API.

  Store with store_memory (or supersede_memory if an earlier DEC-05 record exists); an executor
  without engram MCP access leaves this to the orchestrator.
```
