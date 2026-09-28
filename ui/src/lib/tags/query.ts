// The one cached ListTags query per scope key (D-13, D-15..D-19): TagBars and
// TagCombobox share this query so a scope key is ever fetched once, regardless
// of how many surfaces render its tags. limit is fixed at TAGS_LIMIT (1000,
// the server maximum) -- callers slice/filter the cached response client-side
// rather than requesting a narrower page (D-15's "one fetch" pattern).
import { engram } from '$lib/client';

// Sentinel for "every readable scope" in the query key -- ListTags itself
// takes an empty string for this meaning; the sentinel exists only so the key
// is never an empty-string tuple slot (easy to typo-collide with an actual
// scope that happens to stringify empty).
export const ALL_READABLE_SCOPES = '__all_readable__';

export const TAGS_LIMIT = 1000;

export function listTagsKey(scope: string) {
  return ['listTags', scope || ALL_READABLE_SCOPES, TAGS_LIMIT];
}

export function listTagsQuery(scope: string) {
  return {
    queryKey: listTagsKey(scope),
    queryFn: ({ signal }: { signal: AbortSignal }) =>
      engram.listTags({ scope, limit: BigInt(TAGS_LIMIT) }, { signal }),
    staleTime: Infinity,
    meta: { silent: true }
  };
}
