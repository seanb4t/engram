export const PAGE_LIMIT = 50;

export type Category = 'convention' | 'gotcha' | 'decision' | 'preference';

export const CATEGORIES: readonly Category[] = ['convention', 'gotcha', 'decision', 'preference'];

/** Recognised `inc` URL-parameter values, in canonical order. Shared by parse and encode
 * so the two never drift out of sync. */
export const INCLUDE_STATES: readonly string[] = ['archived', 'superseded', 'scheduled'];

export function listMemoriesKey(
  scope: string, categories: string[], visibility: string, limit: number, offset: number,
  includeArchived: boolean, includeSuperseded: boolean, includeScheduled: boolean, crossSpine: boolean
) {
  return ['listMemories', scope, categories, visibility, limit, offset, includeArchived, includeSuperseded, includeScheduled, crossSpine];
}
