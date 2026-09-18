# Phase 4: Drift Detection (Read-Only) - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-15
**Phase:** 4-Drift Detection (Read-Only)
**Areas discussed:** The `preserved` predicate, The live-verify prerequisite, Unparseable / coarse rows

---

## The `preserved` predicate

### Q1 — What makes an observed registration `preserved` rather than `would-write`?

| Option | Description | Selected |
|--------|-------------|----------|
| Anything unaccounted-for | preserved iff the observed entry carries a facet the current Options do not account for; every facet mapping to something setup authors → would-write | ✓ |
| Only inexpressible facets | preserved only when setup literally cannot express the facet; an extra header would be would-write | |
| Unaccounted-for, minus explicit intent | Facets named on argv count as intent and stay would-write; only unmentioned facets can trigger preserved | |

**User's choice:** Anything unaccounted-for (the recommended option).
**Notes:** Conservative posture — the operator's hand-added gateway header is never silently dropped, which is the 2026-09-10 incident (`ryr82bf2s2`) this milestone exists to prevent.

### Q2 — How do header values get compared, given they must be redacted unconditionally?

| Option | Description | Selected |
|--------|-------------|----------|
| Compare raw in memory, redact into the result | Equality-compare raw in a local; carry only the redacted form into result/render/JSON/logs; redaction unconditional regardless of shape | ✓ |
| Redact at the probe boundary | Redact as bytes leave the subprocess; all values compare equal; value drift invisible | |
| Redact at boundary; unverifiable value → preserved | Any observed header is unaccounted-for → preserved; every registration with a header permanently preserved | |

**User's choice:** Compare raw in memory, redact into the result.
**Notes:** Asymmetry noted and accepted — only the observed side needs redaction; the planned value is a bare `${VAR}` name engram authored from argv and renders in full.

### Q3 — What happens to `Result.Registered`, the raw probe capture that currently renders literal header values?

| Option | Description | Selected |
|--------|-------------|----------|
| Replace with parsed-and-redacted fields | Never retain raw probe text; rebuild Registered from the normalized struct — redaction by construction | ✓ |
| Keep raw, add a redacting filter | Keep the bounded raw capture and add a regex redaction pass before displayCapture | |
| Both: parsed field + raw behind a flag | Parsed rendering by default; raw behind an explicit opt-in with a cleartext warning | |

**User's choice:** Replace with parsed-and-redacted fields.
**Notes:** Accepted cost — `--output json`'s `registered` content changes and unrecognized-but-harmless chrome stops being shown.

### Q4 — Where does `preserved` land in the exit taxonomy?

| Option | Description | Selected |
|--------|-------------|----------|
| Non-failed attempt — exit 0 | preserved joins already-correct/would-write/wrote; all-preserved exits 0; 8 only beside a real failure | ✓ |
| New dedicated exit code | A fourth class (e.g. 10) meaning "completed, but at least one registration left untouched" | |
| Partial — exit 8 | Treat preserved as non-convergence and exit 8 | |

**User's choice:** Non-failed attempt — exit 0.
**Notes:** `Classify`'s `default` branch treats an unplaced Outcome as a failure, so the placement must be explicit.

**Continue prompt:** "Next area" selected after four questions.

---

## The live-verify prerequisite

### Q1 — How does the blocking live-verification of literal header echo get satisfied?

| Option | Description | Selected |
|--------|-------------|----------|
| Manual observation, recorded before code is trusted | Maintainer runs a documented protocol once (throwaway entry with a literal header, read verb, capture, remove); fixtures built from the record; no test invokes a real CLI | ✓ |
| Scripted throwaway probe in the plan | A checked-in seed/read/capture/remove script against a temp HOME, run by a human | |
| Fixture-only with a stated assumption | Build fixtures from the documented/likely shape and note the assumption | |

**User's choice:** Manual observation, recorded before code is trusted.
**Notes:** Keeps rule `m45p2b4bp7` intact and matches the `2026-08-23.01` D-10 pattern.

### Q2 — How hard a gate is the observation — what can be built before it lands?

| Option | Description | Selected |
|--------|-------------|----------|
| Soft gate — only the parsers block | All shape-independent work proceeds; only each runtime's scanner and fixtures wait | ✓ |
| Hard gate — observation is task 1 | Nothing written until the observation is recorded | |
| Gate only the redaction proof | Parsers from best-guess shapes; only the REQ-drift-redaction fixture waits | |

**User's choice:** Soft gate — only the parsers block.

### Q3 — Which cases should the one manual observation run capture? (multi-select)

| Option | Description | Selected |
|--------|-------------|----------|
| Literal header value echo (required) | The ROADMAP's named blocker on claude and codex | ✓ |
| Bare-reference header echo | The `${VAR}`/`{env:VAR}` shape read back | |
| Absent / no-registration read | What each read verb prints/exits with no entry | |
| OAuth-authenticated registration | Read back an entry that completed OAuth login | |

**User's choice:** Literal header value echo only.
**Notes:** Bare-reference echo already observed in research; absent-read already exercised by today's probe handling; OAuth read-back belongs to Phase 5.

### Q4 — Where does the recorded observation live?

| Option | Description | Selected |
|--------|-------------|----------|
| Phase artifact, cited by each fixture | Dated `04-OBSERVATIONS.md` with verbatim output, commands, CLI versions; each fixture cites it | ✓ |
| Header comments in the fixtures only | Provenance comment per fixture, no separate document | |
| Phase artifact plus an engram gotcha | The artifact plus a durable engram memory record | |

**User's choice:** Phase artifact, cited by each fixture.

**Continue prompt:** "Next area" selected after four questions.

---

## Unparseable / coarse rows

### Q1 — When a runtime's registration can't be parsed, what does it classify as?

| Option | Description | Selected |
|--------|-------------|----------|
| would-write — ambiguity never becomes preserved | Extends the existing "ambiguity resolves to wrote, never already-correct" invariant; preserved is always a positively-observed, nameable claim | ✓ |
| preserved — unreadable means hands off | If setup cannot prove non-destructive, it does not write; opencode could never re-register | |
| Split by cause | Clean slate → would-write; existing-but-unreadable → preserved | |

**User's choice:** would-write — ambiguity never becomes preserved.

### Q2 — What shape does opencode's documented coarse comparison actually take?

| Option | Description | Selected |
|--------|-------------|----------|
| No comparison; always would-write, documented | opencode authors no classification; guide states it is not compared; SC2 three-state coverage read as parsed runtimes | ✓ |
| Coarse name-presence, reported not classified | D-11-style name match surfaced as a note; outcome stays would-write | |
| Parse opencode after all | Bring REQ-drift-opencode-structured into scope | |

**User's choice:** No comparison; always would-write, documented.
**Notes:** Spec tension surfaced — SC2 says "per runtime" but opencode structurally cannot reach three states under D-09; resolved by documenting the exemption in the phase artifacts.

### Q3 — How strict is Claude Code's bounded text scan about content it doesn't recognize?

| Option | Description | Selected |
|--------|-------------|----------|
| Total parse; unrecognized content → preserved | Every line in the facet-bearing region must map to a known facet; leftovers trigger preserved naming the (redacted) content | ✓ |
| Tolerant — compare known facets only | Scan for known lines; ignore the rest as chrome | |
| Strict region, tolerant chrome | Allowlisted decorative lines ignored; entry body parsed totally | |

**User's choice:** Total parse; unrecognized content → preserved.
**Notes:** The tolerant option was flagged as silently defeating D-01 — anything the scanner was not taught to see cannot protect the operator.

### Q4 — How is the differing facet represented — and what happens when several differ at once?

| Option | Description | Selected |
|--------|-------------|----------|
| Closed typed enum, all differing facets in stable order | Typed `Facet` (url, auth-mode, header-name, header-value-ref, unrecognized-content); every differing facet in fixed order; rendered from the typed set in text and JSON | ✓ |
| Typed enum, first difference only | Same vocabulary, only the first facet in precedence order | |
| Free-text reason string | A composed human sentence on a Reason-style field | |

**User's choice:** Closed typed enum, all differing facets in stable order.

**Continue prompt:** "Next — wrap up" selected after four questions.

---

## Readiness check

**Prompt:** "We've covered the preserved predicate, live-verify, and coarse rows. Which gray areas remain unclear?"
**User's choice:** I'm ready for context.

## Claude's Discretion

- Where the comparison lives (per-runtime scanner in each runtime's own file + a pure `drift.go` predicate/diff vs. widening the `Runtime` interface).
- The Codex JSON totality mechanism (`json.Decoder.DisallowUnknownFields` is the natural fit).
- What a `preserved` row renders for its planned argv.
- The redacted placeholder text and how the typed facet set folds into flat scalar row fields.
- How Phase 3's plugin facet sits beside a `preserved` registration on the same row.
- Exact `guides/agent-setup.md` results-table wording, including the Codex whole-entry sentence.
- Recognizing the Claude Code probe's connection-status line as chrome under D-11 (the probe dials the registered URL).

## Deferred Ideas

- Full-fidelity opencode comparison (`REQ-drift-opencode-structured`) — revisit if opencode gains a `--json` read verb.
- Bare-reference / absent-read / OAuth read-back observations — offered for the same sitting and declined; OAuth belongs to Phase 5.
- A durable engram gotcha for the observed echo behavior — offered and declined in favor of the phase artifact.
