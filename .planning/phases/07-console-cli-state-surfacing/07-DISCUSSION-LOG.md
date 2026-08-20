# Phase 7: Console & CLI State Surfacing - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-20
**Phase:** 7-Console & CLI State Surfacing
**Areas discussed:** Reaching hidden records, Migration state transport, CLI shape and renderer, State vocabulary & placement

## Reaching hidden records

### Q1 — How does an operator reach a superseded, archived, or not-yet-active record?

| Option | Description | Selected |
|---|---|---|
| Opt-in include on list/search | New request fields relax the IsEmpty gates; the only option where an archived record is reachable at all, since nothing links to one. Cost: proto, four gate sites, re-proving the recall-gate reachability derivation | ✓ |
| Presentation-only | Gate untouched; state renders where a record is already reachable (detail pane via GetMemory, json lane, new `engram get`). Archived records stay invisible | |
| Follow-the-link navigation | superseded_by/supersedes render as links; chains walkable from the live head. Does nothing for archived or scheduled | |

**User's choice:** Opt-in include on list/search
**Notes:** Sizes the phase as a store + proto change, not a UI pass. Flagged that `schemaversion_recallgate_test.go`'s reachability derivation and `backlogFilter`'s "never reachable from any recall entry point" claim both have to be re-run, not assumed.

### Q2 — What shape does the opt-in take on the request messages?

| Option | Description | Selected |
|---|---|---|
| Three orthogonal bools | include_archived / include_superseded / include_scheduled, 1:1 with the three gates store.go refuses to fold; composable | ✓ |
| Repeated state enum | One `repeated RecordState` per RPC; one field, extensible, closest to list_scheduled's vocabulary. Cost: a flag→condition mapping table | |
| Single include_hidden bool | Smallest surface. Cannot express "just the archive tier", which is what SC1 names | |

**User's choice:** Three orthogonal bools

### Q3 — Which lanes get the flags?

| Option | Description | Selected |
|---|---|---|
| Connect + CLI only | Store options land once and both lanes call them; only Connect and the CLI expose the flags. MCP recall stays zero-junk; later MCP exposure is additive | ✓ |
| All three lanes | Makes "every lane" literally true. Cost: an agent seeing superseded records in recall, which the memory contract currently forbids | |
| Console only | Skips CLI flags entirely. Contradicts SC2 | |

**User's choice:** Connect + CLI only

### Q4 — Does another authenticated caller see a shared record that is superseded/archived?

| Option | Description | Selected |
|---|---|---|
| Yes — authz stays orthogonal | Flags relax only state gates; owner/shared filter untouched. Coupling would push authz into the recall filter | ✓ |
| No — hidden state is owner-only | Nothing invisible today becomes visible to a third party. Cost: a new authz coupling and a "shared but hidden is private" rule to document everywhere | |
| You decide | Defer to research/planning against the cedar policy set | |

**User's choice:** Yes — authz stays orthogonal

## Migration state transport

### Q1 — How does pending-migration state reach the console?

| Option | Description | Selected |
|---|---|---|
| New Connect RPC | Same generated client, auth chain, and error envelope as every other call; also gives the CLI a client-tier path, since `engram migrate status` is operator-tier and needs direct Qdrant access | ✓ |
| Plain HTTP endpoint | Avoids proto. Cost: a second transport with its own auth, error shape, and hand-written TS type | |
| Derive client-side | Zero server work. Measures only the current page/scope and cannot see the absent-key legacy bucket — wrong in a way that looks right | |

**User's choice:** New Connect RPC

### Q2 — Who may call it?

| Option | Description | Selected |
|---|---|---|
| Any authenticated caller | Version buckets plus totals, no content. Discloses aggregate collection size; answers the operator's actual whole-collection question | ✓ |
| Owner-scoped histogram | No cross-owner disclosure. Cost: can read zero while a large legacy backlog exists | |
| Both — scoped plus totals | Honest about both numbers. Cost: two counting paths, and the disclosure question is moved rather than avoided | |

**User's choice:** Any authenticated caller
**Notes:** Surfaced that admin-only is not a cheap alternative — `internal/authz/entities.go:42-47` intentionally omits `roles` as a forward-compat reservation.

### Q3 — Where does it appear in the console?

| Option | Description | Selected |
|---|---|---|
| AppShell banner when non-zero | Every route, silent at zero; mirrors the startup slog.Warn's intent. Distinct treatments for behind-version vs ahead-version | ✓ |
| Observe-page panel | Keeps diagnostics in one place. Cost: an operator who never opens /observe never learns of a backlog | |
| Sidebar indicator that expands | Always present, never intrusive. Cost: easy to miss when it matters; needs a healthy-state visual | |

**User's choice:** AppShell banner when non-zero

### Q4 — What is the CLI's client-tier surface?

| Option | Description | Selected |
|---|---|---|
| Verb + advisory footer | Verb makes it actionable, footer satisfies "not only by running a command directly". Cost: one extra RPC per search/list | ✓ |
| Dedicated verb only | No hot-path cost. Cost: the operator still has to know to run it | |
| Advisory footer only | Purely passive. Cost: nowhere to see the per-version distribution | |
| Piggyback a field on existing responses | Free at runtime. Cost: permanent coupling of an unrelated concern into two hot response messages | |

**User's choice:** Verb + advisory footer

## CLI shape and renderer

### Q1 — SC2 names `engram get`, which doesn't exist. What happens?

| Option | Description | Selected |
|---|---|---|
| Add it | Over the existing GetMemory RPC; the only ungated read path, so SC2 naming it was not an accident | ✓ |
| Reinterpret SC2 as search/list only | Smaller phase. Cost: no CLI detail view at all, leaving the asymmetry SC2 exists to close | |
| Add it, and correct SC2 wording too | Same work plus an explicit roadmap correction via /gsd-phase edit | |

**User's choice:** Add it
**Notes:** The roadmap-correction path is recorded in CONTEXT.md's canonical refs regardless — Phase 5 needed two such edits and the handling is established.

### Q2 — Does the client tier adopt Phase 6's typed view, and how far?

| Option | Description | Selected |
|---|---|---|
| Detail view only; table stays a table | `engram get` renders through renderOperatorView via a protojson-bytes adapter; search/list keep renderMemoryTable | ✓ |
| Port the whole client tier | One mechanism across both tiers. Cost: N results become N stacked field tables; columnar scannability lost | |
| Extend renderMemoryTable in place | Smallest. Cost: SC2's "typed renderer" claim becomes false | |

**User's choice:** Detail view only; table stays a table
**Notes:** Surfaced during discussion that `viewFields` marshals via `json.Marshal`, which returns a `json.RawMessage` verbatim — so the protojson adapter is one line. Phase 6 assumed this port would be costlier.

### Q3 — Does this phase close the headline sanitization hole (#505)?

| Option | Description | Selected |
|---|---|---|
| Fix it structurally, now | Route the headline through sanitizeViewValue. This phase creates the exploit condition, making the fix a precondition rather than smuggled scope | ✓ |
| Keep the invariant, document it | Constrain the headline to server-minted values. Cost: scope and tags are user-supplied, owner is IdP-supplied — "safe values" is narrower than it looks | |
| No headline on the client tier | Sidesteps it. Cost: #505 stays open; loses the at-a-glance line D-04 argued for | |

**User's choice:** Fix it structurally, now
**Notes:** Durable record `5dr8amcx1w` is tagged `phase-07-will-reach-it` — this discussion confirmed that prediction against source.

### Q4 — What does the compact table show for state?

| Option | Description | Selected |
|---|---|---|
| STATE column only when a flag is set | Default output byte-identical to today; reuses the withScore conditional-column pattern | |
| Always-present STATE column | One stable table shape regardless of flags. Cost: changes today's default output for every user | ✓ |
| Marker on SHORT_ID | No width cost. Not composable; reads as decoration | |
| Nothing in the table | Table untouched. Cost: include_archived becomes useful only in json | |

**User's choice:** Always-present STATE column — **overrides the recommended conditional-column option**
**Notes:** Phase 6 D-03 makes this contractually free — `--output text` is explicitly not a stable interface. The json lane is untouched either way.

## State vocabulary & placement

### Q1 — How do the surfaces agree on the vocabulary?

| Option | Description | Selected |
|---|---|---|
| The wire is the vocabulary | Both derive the label from the same proto field being set; gated with a test per surface | ✓ |
| Generate a shared string table | Provable byte-identity. Cost: a fourth generated tree for three short words | |
| Convention plus a docs anchor | Zero machinery. Cost: enforced by nothing — the shape that produced #505 | |

**User's choice:** The wire is the vocabulary

### Q2 — How does state render in MemoryDetail.svelte?

| Option | Description | Selected |
|---|---|---|
| Dedicated State section | Renders what chips cannot: superseded_by as a link, timestamps as timestamps. schema_version always present per Phase 2 D-10 | ✓ |
| Extend the existing chip row | Consistent, zero new layout. Cost: a chip cannot carry a target id — the operator learns a record was superseded but not by what | |
| Header badge next to category | Most prominent. Cost: same expressiveness ceiling; competes with the category color system | |

**User's choice:** Dedicated State section

### Q3 — How does a hidden record read in a list row?

| Option | Description | Selected |
|---|---|---|
| Explicit marker in the meta line | Direct parallel of the CLI's STATE column; no color-only signalling | |
| Dimmed row treatment | Scannable. Cost: appearance-only, and says "deprioritized" about records the operator asked for | |
| Marker plus dimming | Marker names the state, dimming separates the groups. Cost: two mechanisms for one fact; needs a rule for archived AND superseded | ✓ |

**User's choice:** Marker plus dimming — **overrides the recommended marker-only option**
**Notes:** Two constraints recorded into CONTEXT.md D-15: the dimming stays strictly decorative on top of the marker (the marker is what a screen reader gets), and "archived AND superseded" needs a defined dimming rule since the flags are orthogonal.

### Q4 — Where does the console's include control live?

| Option | Description | Selected |
|---|---|---|
| ScopesSidebar with the other filters | Where every other filter lives; inherits URL round-trip and query-cache keying for free | ✓ |
| Separate control above the list | Marks it as a different kind of filter. Cost: a second filter location and new URL/cache plumbing | |
| URL parameters only | Zero console work. Undiscoverable — fails SC1's premise | |

**User's choice:** ScopesSidebar with the other filters

## Claude's Discretion

- Names of the new RPC, its request/response messages, and the histogram bucket shape.
- The client-tier migration verb's name, resolved against existing verb naming and the toolclass registry.
- Field numbers for the six new request fields; whether `cross_spine` composition needs an explicit guard.
- Whether `engram get` accepts multiple ids and whether it needs a `--full` flag.
- How the footer's extra RPC is scheduled (a failed lookup must never fail the command).
- Whether the banner polls, refetches on route change, or fetches once per session.
- Exact wording of the two banner conditions and the footer line.
- Whether STATE renders one composed value or a fixed-width flag set for a record carrying two states.
- Plan ordering — split by tier or by capability.

## Deferred Ideas

- The three include flags on the MCP tool schemas — additive later; store options land in this phase.
- Porting `search`/`list` off `renderMemoryTable` onto the view mechanism — Phase 6's original deferred idea, still open for the multi-record case.
- Piggybacking a pending-migration count onto the hot list/search responses — revisit only if the footer's extra RPC proves measurably costly.
- An admin/operator role for the migration-status RPC — a first candidate if a later milestone populates `roles`.
- Hardening the red-evidence harness (`366pjeht8e`) — inherited from Phase 6, still test infrastructure this phase does not cause.

### Reviewed Todos (not folded)

- **Research a versioned payload-migration mechanism** (score 0.6) — declined for the third consecutive phase. STATE.md records it as consumed by Phases 2-4, all complete. Keyword noise.
