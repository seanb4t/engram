---
status: complete
phase: 01-store-prerequisites
source: [01-VERIFICATION.md]
started: 2026-09-26T04:26:29Z
updated: 2026-09-26T11:34:49Z
---

## Current Test

[testing complete]

## Tests

### 1. Concurrent ArchiveAs/RestoreAs on one id
expected: Record ends in exactly one of the two states, never torn
result: pass — automated: TestArchiveAsRestoreAsConcurrentSerialize (commit 14c7aa5e)

### 2. Concurrent write during ListTags facet read
expected: Facet count never exposes an unreadable record
result: pass — automated: TestListTagsUnderConcurrentWrites (commit 14c7aa5e)

### 3. Candidate disappearing between RelatedMemories' vector Query and payload fetch
expected: Candidate silently dropped, no error, no distinguishable truncation signal
result: pass — automated: TestRelatedMemoriesCandidateVanishesBeforeFetch (commit 14c7aa5e)

### 4. 01-03's judgment-tier prohibition: edge types are never folded into one comparable score or re-ranked across types
expected: RelatedEdge keeps per-type evidence fields only; assembleRelated admits by fixed relatedEdgeRank order, never a blended score
result: pass — explicit human sign-off (Sean, 2026-09-26); behavioral evidence TestRelatedMemoriesAdmitsByTypeNotScore (commit 14c7aa5e)

## Summary

total: 4
passed: 4
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps
