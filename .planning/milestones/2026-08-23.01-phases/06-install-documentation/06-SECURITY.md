---
phase: 06
slug: install-documentation
status: verified
threats_open: 0
asvs_level: 1
block_on: high
created: 2026-09-12
---

# Phase 6 — Security

The independent GSD security auditor verified the eight plan-authored threats
at ASVS L1. All eight mitigations are present; no blocking or non-blocking
threat remains open in this register. The scope is D-10 pre-merge acceptance.
Qualifying setup release verification remains pending in `06-POST-RELEASE.md`.

## Trust boundaries

Credential examples cross chat/shell boundaries; setup preview crosses into
runtime configuration only under explicit apply; published artifacts execute
inside disposable installations; observation records support acceptance claims.

## Threat register

| Threat | Category | Severity | Disposition | Verified mitigation | Status |
| --- | --- | --- | --- | --- | --- |
| T-06-01 | Information disclosure | high | mitigate | Agent Setup auth section and Plugin preserve non-secret client IDs, inherited secrets, literal token references and generic-only token files; CLI distinguishes Connect credentials | closed |
| T-06-02 | Tampering | high | mitigate | Agent Setup preview/review/apply ordering and Plugin confirmation; install harnesses invoke only version/help, never registration | closed |
| T-06-03 | Spoofing | medium | mitigate | Install uses canonical tap/release URLs, checksums before extraction, version checks and executable-scoped quarantine removal | closed |
| T-06-04 | Repudiation | medium | mitigate | Dated unreleased warnings remain in all relevant guides; post-release handoff gates their removal on actual evidence | closed |
| T-06-05 | Tampering | high | mitigate | Inspected harnesses assert disposable prefix/Caskroom/binary paths, scoped cache/log/temp, non-root containers without host mounts; cleanup results confirm task-resource removal | closed |
| T-06-06 | Spoofing | high | mitigate | Auditor compared all four decoded cask SHA-256 values with release metadata and checked upload/push logs, installed versions, tap revision and architecture identities | closed |
| T-06-07 | Repudiation | medium | mitigate | Release observations retain commands, failures and execution qualifications, and distinguish successful installation from missing setup | closed |
| T-06-08 | Elevation of privilege | high | mitigate | Release-please owns tags; handoff prohibits fabricated release evidence; current ship request covers PR preparation and does not assert a release | closed |

## Evidence

The auditor inspected the five guide sources, both plans and summaries, clean
`06-REVIEW.md`, `06-RELEASE-OBSERVATIONS.md`, D-10 and `06-POST-RELEASE.md`.
It also inspected root's actual macOS/Linux harnesses and four install transcripts,
combined results, decoded cask/release metadata, release run log and cleanup result
under `/tmp/engram-06-*`. The observations artifact preserves the essential
identities and results after disposable environments were removed.

No new threat scan, install repetition, runtime registration, user-home mutation
or release action was performed by the auditor. Neither summary supplied a
Threat Flags section; that omission is not proof of absent unrelated attack surface.

## Accepted risks

None. No finding was waived to satisfy the shipping gate.

## Audit trail

| Date | Auditor | Total | Closed | Open |
| --- | --- | --- | --- | --- |
| 2026-09-12 | gsd-security-auditor, ship6_security | 8 | 8 | 0 |

Approval: verified for pre-merge scope, 2026-09-12.
