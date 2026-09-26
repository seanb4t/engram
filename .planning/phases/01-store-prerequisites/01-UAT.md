---
status: testing
phase: 01-store-prerequisites
source: [01-VERIFICATION.md]
started: 2026-09-26T04:26:29Z
updated: 2026-09-26T04:26:29Z
---

## Current Test

number: 1
name: Concurrent ArchiveAs/RestoreAs on one id
expected: |
  Record ends in exactly one of the two states, never torn
awaiting: user response

## Tests

### 1. Concurrent ArchiveAs/RestoreAs on one id
expected: Record ends in exactly one of the two states, never torn
result: [pending]

### 2. Concurrent write during ListTags facet read
expected: Facet count never exposes an unreadable record
result: [pending]

### 3. Candidate disappearing between RelatedMemories' vector Query and payload fetch
expected: Candidate silently dropped, no error, no distinguishable truncation signal
result: [pending]

### 4. 01-03's judgment-tier prohibition: edge types are never folded into one comparable score or re-ranked across types
expected: RelatedEdge keeps per-type evidence fields only; assembleRelated admits by fixed relatedEdgeRank order, never a blended score
result: [pending]

## Summary

total: 4
passed: 0
issues: 0
pending: 4
skipped: 0
blocked: 0

## Gaps
