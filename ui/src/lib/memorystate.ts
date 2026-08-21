import type { Memory } from '$lib/gen/engram_pb';
import { timestampDate } from '@bufbuild/protobuf/wkt';

// The console's ONLY state-word derivation (D-13). It reads the same wire
// fields the Go surface's memoryStateWords (cmd/engram/memory_state.go)
// reads, so neither surface can render a word the other cannot express —
// deliberately NOT a shared table, each surface has its own test proving
// the derivation.
export type RecordStateWord = 'archived' | 'superseded' | 'expired' | 'scheduled';

// Canonical order (descending operator-action finality, per the UI-SPEC):
// archived is a deliberate terminal action, superseded has a live successor
// to follow, expired is a lapsed window nobody corrected, scheduled is the
// least severe (a live record, early).
export const STATE_WORD_ORDER: readonly RecordStateWord[] = ['archived', 'superseded', 'expired', 'scheduled'];

// Boundary comparisons mirror internal/store's activeWindowConditions
// exactly: not_before uses an inclusive Lte bound (not_before === now is
// already active, no "scheduled" word), not_after uses an exclusive Gt bound
// (not_after === now is already expired, "expired" word).
//
// expired/scheduled precedence is defined HERE, in the derivation, not
// inherited from write-time validation. RuleWindowOrdering forbids WRITING
// not_before >= not_after, but notBefore and notAfter are independent wire
// fields (proto/engram/v1/engram.proto:46-47) and a legacy or future-path
// record can arrive with an inverted window. expired is evaluated first and,
// when emitted, suppresses scheduled — the exact precedence
// cmd/engram/memory_state.go's memoryStateWords uses, which is what keeps
// the two surfaces in agreement on an inverted-window input.
// The returned array is ordered by filtering STATE_WORD_ORDER rather than by
// the order the applicability checks below happen to run in. That makes the
// exported constant load-bearing: reordering it reorders every consumer's
// rendered badges, and no caller has to remember to sort. Callers therefore
// never order state words themselves — they render what this returns.
export function memoryStateWords(m: Memory, now: Date = new Date()): RecordStateWord[] {
  const applicable = new Set<RecordStateWord>();
  if (m.archivedAt) applicable.add('archived');
  if (m.supersededBy) applicable.add('superseded');
  let expired = false;
  if (m.notAfter && timestampDate(m.notAfter).getTime() <= now.getTime()) {
    applicable.add('expired');
    expired = true;
  }
  if (!expired && m.notBefore && timestampDate(m.notBefore).getTime() > now.getTime()) {
    applicable.add('scheduled');
  }
  return STATE_WORD_ORDER.filter((word) => applicable.has(word));
}

// A record is dimmed iff it is PAST — archived, superseded, or expired.
// scheduled describes a record's future, so it never dims and never
// suppresses a dim contributed by another state.
export function isPastState(words: RecordStateWord[]): boolean {
  return words.includes('archived') || words.includes('superseded') || words.includes('expired');
}
