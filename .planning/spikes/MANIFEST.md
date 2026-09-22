# Spike Manifest

## Ideas

### jev-typed-decisions

Use TypeSafe's Jev (System One typed-decision model) behind a provider-neutral,
advisory-only decision interface in engram, for spine-curation verdicts, recall reranking,
and write-time hints. Context: `.planning/notes/jev-system-one-decisions.md`,
`.planning/seeds/typed-decision-provider.md`.

**Requirements:**

- Provider-neutral interface using System One vocabulary (state + Choice/Score/Noul →
  probabilities); Jev is one backend, a chat-LLM emulator another.
- Off by default; advisory only — verdicts are surfaced, never acted on (reranking allowed).
- Transport goes through `/api/alpha/decisions`; engram's chat client cannot reach Jev.

## Spikes

| # | Idea | Name | Type | Validates | Verdict | Tags |
|---|------|------|------|-----------|---------|------|
| 001 | jev-typed-decisions | jev-openrouter-transport | standard | Given the OpenRouter key, when a Decisions request is POSTed, then typed probabilities return with usable latency and known limits | VALIDATED | jev, openrouter, decisions-api, latency |
