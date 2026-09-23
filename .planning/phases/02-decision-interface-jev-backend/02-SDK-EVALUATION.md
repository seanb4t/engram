sdk_module: github.com/OpenRouterTeam/go-sdk
sdk_version: v0.8.19
legitimacy: approved
verdict: PENDING
resolution: PENDING

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

Pending plan 02-02.

## Verdict (plan 02-02)

Pending plan 02-02.

## Durable decision record (plan 02-02)

Pending plan 02-02.
