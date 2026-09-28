// The header search hand-off store (D-11): the ⌘K command menu (Phase 3,
// 02-05) hands typed text to the header search through this module rather
// than searching memories itself — the header search is the only place
// memories are searched (ENTRY-04).

export const headerSearch = $state({ text: '', focusSeq: 0 });

export function handoffToHeaderSearch(text: string): void {
  headerSearch.text = text;
  headerSearch.focusSeq += 1;
}

export function focusHeaderSearch(): void {
  headerSearch.focusSeq += 1;
}
