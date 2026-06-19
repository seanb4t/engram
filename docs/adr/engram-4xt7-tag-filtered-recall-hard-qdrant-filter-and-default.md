<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-4xt7; do not edit manually; use `/adr update engram-4xt7` -->

# Tag-filtered recall: hard Qdrant filter, AND-default

**Date:** 2026-06-19
**Status:** Accepted
**Decision:** engram-4xt7
**Deciders:** Sean Brandt

## Context

Tags are persisted on every record but, before this change, participated in recall through NEITHER mechanism: the embedder only ever sees content, and Store.Search / Store.List built Qdrant filters on scope/owner/visibility/window but never on tags. So tags were doubly presentational. engram-4sw made tags mutable on the write path; engram-6ec makes them a recall dimension on the read path. The curating-memory design is deliberately semantic-first / zero-junk (agents should not have to manage a tag taxonomy), so turning tags into a recall filter is a conscious softening of that stance and warrants a recorded decision.

## Decision

search_memory and list_memory gain an optional tags filter. It is implemented as one Qdrant exact-match Must condition per requested tag (tagMatchConditions); because Qdrant matches a scalar against a list-valued payload field by membership, N Must conditions mean 'carries ALL N tags' (AND / contains-all). On search_memory the filter is a hard pre-filter applied before ANN vector ranking; on list_memory it composes with the existing recency sort. No match-mode / OR override ships (AND only). No payload index is added. The Connect read API (EngramService) is unchanged and passes nil.

## Rationale

A hard boolean filter gives precise scoping for operator-console and targeted recall, which a fuzzy signal cannot. AND is the precise-scoping case; OR is rarely needed and addable later. Per-tag Must composes orthogonally with the existing owner/visibility/window Must envelope, so a tag filter can only NARROW a caller's already-authorized result set — it never bypasses authz. Not adding an index keeps tags consistent with scope/owner/category, which all already filter unindexed; Qdrant supports unindexed payload filtering and the store is small.

## Alternatives Considered

(1) Soft semantic signal — fold tags into the text embedded at store/update time so they nudge similarity. REJECTED: pollutes the content vector, is a weak/fuzzy signal, and cannot be queried precisely. (2) Ship a match-mode/OR toggle up front. DEFERRED (YAGNI): AND covers the precise-scoping need; OR can be added without changing existing behavior. (3) Do not build it — keep recall purely semantic. REJECTED: tags would stay doubly presentational and engram-4sw's mutable tags would have no recall payoff. (4) Add a keyword payload index on tags. DEFERRED: no payload indexes exist today and none are needed at current scale; addable later without contract change.

## Consequences

Positive: tags become a usable recall dimension on both read paths; fully backward compatible (filter is optional, omitting it is a passthrough); the filter lives in the store layer composed onto the authz/window Must envelope, so no handler can bypass isolation. Negative: softens the semantic-first / no-taxonomy stance and invites some taxonomy management — mitigated by updated curating-memory guidance favoring a few stable, low-cardinality tags. Unindexed tag filtering is a full scan at scale. Neutral: OR/match-mode and a payload index can each be added later without changing the contract.
