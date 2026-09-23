// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package jev

// Fixture response/error bodies for classify_test.go and jev_test.go.
//
// Fixtures whose provenance comment names a spike probe are extracted
// verbatim via:
//
//	jq -r '[.events[] | select(.probe=="<probe>")][0].body' \
//	  .claude/skills/spike-findings-engram/sources/001-jev-openrouter-transport/results.json
//
// or copied verbatim from spike 002's README. Fixtures marked "synthetic"
// are hand-written to the same wire shapes for statuses the spikes never
// exercised live. No request-state text from the spike's State map is
// copied here — these are response bodies only.

// fixtureOpenRouter401 is spike 001 probe "badkey" (401, numeric code).
const fixtureOpenRouter401 = `{"error":{"message":"Missing Authentication header","code":401}}`

// fixtureOpenRouter400Choices is spike 001 probe "card256" (400, too many
// choices — plain-string detail, not the max_tokens_exceeded shape).
const fixtureOpenRouter400Choices = `{"error":{"message":"HTTP 400: {\"detail\":\"Too many choices. Must have at most 255 choices.\"}","code":400}}`

// fixtureOpenRouter400BadType is spike 001 probe "badtype" (400, zod
// invalid_union — a JSON array, not the max_tokens_exceeded object shape).
const fixtureOpenRouter400BadType = `{"error":{"message":"[\n  {\n    \"code\": \"invalid_union\",\n    \"errors\": [],\n    \"note\": \"No matching discriminator\",\n    \"discriminator\": \"type\",\n    \"options\": [\n      \"noul\",\n      \"choice\",\n      \"score\"\n    ],\n    \"path\": [\n      \"questions\",\n      \"q\",\n      \"type\"\n    ],\n    \"message\": \"Invalid discriminator value. Expected 'noul' | 'choice' | 'score'\"\n  }\n]","code":400},"user_id":"REDACTED"}`

// fixtureOpenRouter400MaxTokens is spike 001 probe "oversize" (400, state >
// 32k tokens; error.message embeds a max_tokens_exceeded JSON document after
// an "HTTP 400: " prefix — the one structural context-too-large shape D-12
// allows).
const fixtureOpenRouter400MaxTokens = `{"error":{"message":"HTTP 400: {\"detail\":{\"error_type\":\"max_tokens_exceeded\"}}","code":400}}`

// fixtureChatPath400 is spike 001 probe "chatpath" (400, Jev slug hit the
// chat/completions endpoint instead of /alpha/decisions).
const fixtureChatPath400 = `{"error":{"message":"typesafe/jev-1.13 is a decisions model and cannot be used with the chat/completions endpoint. Use the /api/alpha/decisions endpoint instead.","code":400},"user_id":"REDACTED"}`

// fixtureLiteLLM401 is spike 002's README-captured LiteLLM pass-through body
// (401, string code — the second error dialect D-12 must classify
// identically to OpenRouter's numeric-code dialect).
const fixtureLiteLLM401 = `{"error":{"message":"Authentication Error, No api key passed in.","type":"auth_error","param":"None","code":"401"}}`

// fixtureLiteLLM403 is synthetic: the same LiteLLM string-code dialect at
// 403 (no live spike probe recorded a 403).
const fixtureLiteLLM403 = `{"error":{"message":"Authentication Error, forbidden.","type":"auth_error","param":"None","code":"403"}}`

// The remaining status fixtures are synthetic OpenRouter-dialect (numeric
// code) bodies at statuses the spikes never exercised live.
const (
	fixtureOpenRouter402 = `{"error":{"message":"Payment required.","code":402}}`
	fixtureOpenRouter404 = `{"error":{"message":"Not found.","code":404}}`
	fixtureOpenRouter413 = `{"error":{"message":"Payload too large.","code":413}}`
	fixtureOpenRouter429 = `{"error":{"message":"Rate limited.","code":429}}`
	fixtureOpenRouter500 = `{"error":{"message":"Internal server error.","code":500}}`
	fixtureOpenRouter502 = `{"error":{"message":"Bad gateway.","code":502}}`
	fixtureOpenRouter503 = `{"error":{"message":"Service unavailable.","code":503}}`
	fixtureOpenRouter524 = `{"error":{"message":"Edge network timeout.","code":524}}`
	fixtureOpenRouter529 = `{"error":{"message":"Provider overloaded.","code":529}}`
)

// fixtureHTML502 is synthetic: a non-JSON body, as a misconfigured proxy in
// front of the gateway might return.
const fixtureHTML502 = `<html><body><h1>502 Bad Gateway</h1></body></html>`

// fixtureNoulOK is a 200 success body carrying one noul answer named "q",
// derived from spike 001's "happy" shape (dated model snapshot, usage with
// cost, id, provider).
const fixtureNoulOK = `{"model":"typesafe/jev-1.13-20260917","answers":{"q":{"type":"noul","noul":0.87}},"usage":{"input_tokens":497,"output_tokens":67,"cost":0.000020874},"id":"gen-fixture-noulok","provider":"TypeSafe"}`

// fixtureNoulNoUsage is fixtureNoulOK with the usage key omitted entirely
// (E10: the usage attributes must be omitted, never zeroed, on this shape).
const fixtureNoulNoUsage = `{"model":"typesafe/jev-1.13-20260917","answers":{"q":{"type":"noul","noul":0.87}},"id":"gen-fixture-noulnousage","provider":"TypeSafe"}`
