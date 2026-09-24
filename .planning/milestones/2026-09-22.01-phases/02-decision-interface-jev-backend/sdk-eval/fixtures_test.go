package sdkeval

// Fixture bodies for the DEC-05 behavioral evaluation (plan 02-02). Verbatim
// fixtures are copied byte-for-byte from the spike sources named in each
// comment via the jq extraction form in 02-02-PLAN.md's <interfaces> block.
// Synthetic fixtures are marked as such and hold no spike-derived text.
//
// None of these bodies carry request-state text derived from real spine
// content — only provider response and error bodies, per the plan's privacy
// prohibition.

// fixtureHappy: verbatim, spike 001 results.json, probe "happy" (200).
const fixtureHappy = `{"model":"typesafe/jev-1.13-20260917","answers":{"relation":{"type":"choice","choice":"duplicate","probabilities":{"unrelated":0,"duplicate":0.92,"related":0.03,"contradicts":0.05},"confidence":0.9},"same_subject":{"type":"noul","noul":0.92}},"usage":{"input_tokens":497,"output_tokens":67,"cost":0.000020874},"id":"gen-dec-1790119657-v90ERBo9Ihr6BnXXvH2K","provider":"TypeSafe"}`

// fixtureBatch50: verbatim, spike 001 results.json, probe "batch50" (200, 50 noul answers).
const fixtureBatch50 = `{"model":"typesafe/jev-1.13-20260917","answers":{"rel_c00":{"type":"noul","noul":0.02},"rel_c01":{"type":"noul","noul":0.03},"rel_c02":{"type":"noul","noul":0.02},"rel_c03":{"type":"noul","noul":0.02},"rel_c04":{"type":"noul","noul":0.03},"rel_c05":{"type":"noul","noul":0.03},"rel_c06":{"type":"noul","noul":0.02},"rel_c07":{"type":"noul","noul":0.03},"rel_c08":{"type":"noul","noul":0.03},"rel_c09":{"type":"noul","noul":0.03},"rel_c10":{"type":"noul","noul":0.03},"rel_c11":{"type":"noul","noul":0.03},"rel_c12":{"type":"noul","noul":0.03},"rel_c13":{"type":"noul","noul":0.03},"rel_c14":{"type":"noul","noul":0.03},"rel_c15":{"type":"noul","noul":0.03},"rel_c16":{"type":"noul","noul":0.03},"rel_c17":{"type":"noul","noul":0.9},"rel_c18":{"type":"noul","noul":0.03},"rel_c19":{"type":"noul","noul":0.03},"rel_c20":{"type":"noul","noul":0.03},"rel_c21":{"type":"noul","noul":0.03},"rel_c22":{"type":"noul","noul":0.04},"rel_c23":{"type":"noul","noul":0.03},"rel_c24":{"type":"noul","noul":0.03},"rel_c25":{"type":"noul","noul":0.04},"rel_c26":{"type":"noul","noul":0.03},"rel_c27":{"type":"noul","noul":0.03},"rel_c28":{"type":"noul","noul":0.04},"rel_c29":{"type":"noul","noul":0.03},"rel_c30":{"type":"noul","noul":0.03},"rel_c31":{"type":"noul","noul":0.03},"rel_c32":{"type":"noul","noul":0.03},"rel_c33":{"type":"noul","noul":0.04},"rel_c34":{"type":"noul","noul":0.03},"rel_c35":{"type":"noul","noul":0.03},"rel_c36":{"type":"noul","noul":0.04},"rel_c37":{"type":"noul","noul":0.04},"rel_c38":{"type":"noul","noul":0.03},"rel_c39":{"type":"noul","noul":0.03},"rel_c40":{"type":"noul","noul":0.04},"rel_c41":{"type":"noul","noul":0.03},"rel_c42":{"type":"noul","noul":0.04},"rel_c43":{"type":"noul","noul":0.04},"rel_c44":{"type":"noul","noul":0.04},"rel_c45":{"type":"noul","noul":0.04},"rel_c46":{"type":"noul","noul":0.04},"rel_c47":{"type":"noul","noul":0.04},"rel_c48":{"type":"noul","noul":0.04},"rel_c49":{"type":"noul","noul":0.03}},"usage":{"input_tokens":4028,"output_tokens":954,"cost":0.000169176},"id":"gen-dec-1790119667-Pr6XXtReMocfPyi196hI","provider":"TypeSafe"}`

// fixtureOpenRouter400Choices: verbatim, spike 001 results.json, probe "card256" (400, >255 choices).
const fixtureOpenRouter400Choices = `{"error":{"message":"HTTP 400: {\"detail\":\"Too many choices. Must have at most 255 choices.\"}","code":400}}`

// fixtureOpenRouter401: verbatim, spike 001 results.json, probe "badkey" (401).
const fixtureOpenRouter401 = `{"error":{"message":"Missing Authentication header","code":401}}`

// fixtureOpenRouter400BadType: verbatim, spike 001 results.json, probe "badtype" (400, zod invalid_union).
const fixtureOpenRouter400BadType = `{"error":{"message":"[\n  {\n    \"code\": \"invalid_union\",\n    \"errors\": [],\n    \"note\": \"No matching discriminator\",\n    \"discriminator\": \"type\",\n    \"options\": [\n      \"noul\",\n      \"choice\",\n      \"score\"\n    ],\n    \"path\": [\n      \"questions\",\n      \"q\",\n      \"type\"\n    ],\n    \"message\": \"Invalid discriminator value. Expected 'noul' | 'choice' | 'score'\"\n  }\n]","code":400},"user_id":"REDACTED"}`

// fixtureOpenRouter400MaxTokens: verbatim, spike 001 results.json, probe "oversize" (400, max_tokens_exceeded).
const fixtureOpenRouter400MaxTokens = `{"error":{"message":"HTTP 400: {\"detail\":{\"error_type\":\"max_tokens_exceeded\"}}","code":400}}`

// fixtureChatPath400: verbatim, spike 001 results.json, probe "chatpath" (400, wrong endpoint).
const fixtureChatPath400 = `{"error":{"message":"typesafe/jev-1.13 is a decisions model and cannot be used with the chat/completions endpoint. Use the /api/alpha/decisions endpoint instead.","code":400},"user_id":"REDACTED"}`

// fixtureLiteLLM401: verbatim, spike 002 README.md "Update — 2026-09-22" section (401, string code).
const fixtureLiteLLM401 = `{"error":{"message":"Authentication Error, No api key passed in.","type":"auth_error","param":"None","code":"401"}}`

// fixtureScore: synthetic: shape from spike 001/002 docs, not captured live.
const fixtureScore = `{"model":"typesafe/jev-1.13-20260917","answers":{"urgency":{"type":"score","score":1,"confidence":0.8,"probabilities":{"0":0.1,"1":0.8,"2":0.1},"legend":{"0":"can wait","1":"this week","2":"blocking"}}},"usage":{"input_tokens":103,"output_tokens":21,"cost":0.0000052},"id":"gen-dec-synthetic-score0001","provider":"TypeSafe"}`

// fixtureLiteLLM403: synthetic: shape from spike 001/002 docs, not captured live.
const fixtureLiteLLM403 = `{"error":{"message":"Forbidden - key lacks the passthrough grant","type":"auth_error","param":"None","code":"403"}}`

// fixtureOpenRouter402: synthetic: shape from spike 001/002 docs, not captured live.
const fixtureOpenRouter402 = `{"error":{"message":"Insufficient credits","code":402}}`

// fixtureOpenRouter404: synthetic: shape from spike 001/002 docs, not captured live.
const fixtureOpenRouter404 = `{"error":{"message":"Model not found","code":404}}`

// fixtureOpenRouter429: synthetic: shape from spike 001/002 docs, not captured live.
const fixtureOpenRouter429 = `{"error":{"message":"Rate limit exceeded","code":429}}`

// fixtureOpenRouter500: synthetic: shape from spike 001/002 docs, not captured live.
const fixtureOpenRouter500 = `{"error":{"message":"Internal server error","code":500}}`

// fixtureOpenRouter502: synthetic: shape from spike 001/002 docs, not captured live.
const fixtureOpenRouter502 = `{"error":{"message":"Bad gateway","code":502}}`

// fixtureOpenRouter503: synthetic: shape from spike 001/002 docs, not captured live.
const fixtureOpenRouter503 = `{"error":{"message":"Service unavailable","code":503}}`

// fixtureOpenRouter524: synthetic: shape from spike 001/002 docs, not captured live.
const fixtureOpenRouter524 = `{"error":{"message":"Edge network timeout","code":524}}`

// fixtureOpenRouter529: synthetic: shape from spike 001/002 docs, not captured live.
const fixtureOpenRouter529 = `{"error":{"message":"Provider overloaded","code":529}}`

// fixtureHTML502: synthetic: shape from spike 001/002 docs, not captured live (a non-JSON gateway error page).
const fixtureHTML502 = `<html><body><h1>502 Bad Gateway</h1><p>gateway.example</p></body></html>`
