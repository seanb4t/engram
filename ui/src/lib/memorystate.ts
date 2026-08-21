import type { Memory } from '$lib/gen/engram_pb';

// The console's ONLY state-word derivation (D-13). STUB — RED phase.
export type RecordStateWord = 'archived' | 'superseded' | 'expired' | 'scheduled';

export const STATE_WORD_ORDER: readonly RecordStateWord[] = ['archived', 'superseded', 'expired', 'scheduled'];

export function memoryStateWords(_m: Memory, _now: Date = new Date()): RecordStateWord[] {
  return [];
}

export function isPastState(_words: RecordStateWord[]): boolean {
  return false;
}
