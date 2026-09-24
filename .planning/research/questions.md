# Research Questions

Open questions that need deeper investigation. Appended by `/gsd-explore`.

## Jev / System One decisions (2026-09-22)

Source: `.planning/notes/jev-system-one-decisions.md`

- **Does the LLM gateway pass through OpenRouter's Decisions API**
  (`/api/alpha/decisions` or `/api/v1/systemone`), not just `/chat/completions`?
  Status: unresolved — unverifiable from docs; test live during the spike.
- **Is zero-data-retention available for Jev, and is there any self-host option?**
  Status: unresolved — only a non-authoritative third-party source claims
  enterprise ZDR on request and no self-host; TypeSafe's own legal pages are
  silent. Confirm with TypeSafe directly.
- **Does any probability signal survive a chat-completions fallback path?**
  Status: unresolved — docs say the chat path 400s for the Jev slug; the
  product surface is about a week old, so re-check.
