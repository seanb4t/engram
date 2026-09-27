<script lang="ts">
  // CUR-03/D-11/D-12: the Rules view. Every readable rule, one read
  // (ListRules with empty scopes), grouped by scope, styled as always-shared,
  // full text on demand via GetMemory. Mirrors /search's route composition
  // (RecallSplit + ResultsList + DetailPane) — see 04-07-SUMMARY.md for the
  // groupKey/rowTrailing plumbing this route consumes.
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { createQuery, keepPreviousData } from '@tanstack/svelte-query';
  import { engram } from '$lib/client';
  import { parseConnectError, fixRowsFor } from '$lib/errors/connect-error';
  import { parseRulesParams, encodeRulesParams, type RulesParams } from '$lib/search/rules-params';
  import { rulesHeaderParts, rulesEmptyHeading, type HeaderPart } from '$lib/search/recall-header';
  import ResultsHeader from '$lib/components/ResultsHeader.svelte';
  import ResultsList from '$lib/components/ResultsList.svelte';
  import RecallSplit from '$lib/components/RecallSplit.svelte';
  import DetailPane from '$lib/components/DetailPane.svelte';
  import RecallState, { type RecallStateInput } from '$lib/components/RecallState.svelte';
  import { Badge } from '$lib/components/ui/badge';

  const params = $derived(parseRulesParams(page.url.searchParams));

  function navigate(next: Partial<RulesParams>) {
    goto(`${base}/rules?${encodeRulesParams({ ...params, ...next })}`, { keepFocus: true, noScroll: true });
  }

  // D-12: one cross-scope read of every readable rule:* scope — empty
  // `scopes` is the all-scopes read (up to 1000 rules total, oldest-first).
  const rulesQ = createQuery(() => ({
    queryKey: ['listRules', [], [], false],
    queryFn: ({ signal }) => engram.listRules({ scopes: [], tags: [], full: false }, { signal }),
    placeholderData: keepPreviousData,
    meta: { silent: true }
  }));

  // Stable group-by-scope sort — ListRules already returns oldest-first
  // within each scope; this only orders the scope groups themselves.
  const rules = $derived.by(() => {
    const list = rulesQ.data?.rules ?? [];
    return [...list].sort((a, b) => (a.scope < b.scope ? -1 : a.scope > b.scope ? 1 : 0));
  });

  // The pane ALWAYS reads the full record via GetMemory — rows are
  // compact (content cleared), unlike /search's memories-first lookup.
  const detailQ = createQuery(() => ({
    queryKey: ['getMemory', params.sel],
    queryFn: ({ signal }) => engram.getMemory({ id: params.sel }, { signal }),
    enabled: !!params.sel,
    meta: { silent: true }
  }));

  const selectedMemory = $derived(detailQ.data?.memory);
  const inResults = $derived(rules.some((m) => m.id === params.sel));

  function closeSel() {
    navigate({ sel: '' });
  }

  function toggleOpen(id: string) {
    if (params.sel === id) closeSel();
    else navigate({ sel: id });
  }

  // D-12: honest header — count, scope coverage, scopes_truncated/unknown —
  // and the server's own advisory line, rendered verbatim never inferred.
  const headerParts = $derived.by((): HeaderPart[] => {
    if (!rulesQ.data) return [];
    return rulesHeaderParts({
      count: rules.length,
      scopeCount: rulesQ.data.searchedScopes?.length ?? 0,
      scopesTruncated: rulesQ.data.scopesTruncated,
      scopesUnknown: rulesQ.data.scopesUnknown
    });
  });

  const advisory = $derived(rulesQ.data?.advisory ?? '');

  // E5 loading: first-load skeleton only; a re-fetch keeps the prior rows
  // (keepPreviousData) and shows the header's progress bar instead.
  const loading = $derived(rulesQ.isLoading && !rulesQ.data);
  const busy = $derived(rulesQ.isFetching && rulesQ.isPlaceholderData);

  const parsedError = $derived(rulesQ.error ? parseConnectError(rulesQ.error) : undefined);
  const isEmpty = $derived(!parsedError && rulesQ.isSuccess && !rulesQ.isFetching && rules.length === 0);

  const recallState = $derived.by((): RecallStateInput | undefined => {
    if (parsedError) return { kind: 'error', parsed: parsedError, fixes: fixRowsFor(parsedError) };
    if (isEmpty) return { kind: 'empty', heading: rulesEmptyHeading(), fixes: [] };
    return undefined;
  });

  // /rules has no facet-driven fixes (no scope/category/k to adjust) — the
  // only fix row fixRowsFor ever returns for a generic rejection is retry.
  function onRecallFix() {
    rulesQ.refetch();
  }

  function onRecallRetry() {
    rulesQ.refetch();
  }
</script>

{#snippet sharedChip()}
  <Badge variant="outline">shared</Badge>
{/snippet}

{#snippet scopeGroupHeader(key: string, count: number)}
  <span class="rules-group-key">{key}</span>
  <span class="rules-group-count">({count})</span>
{/snippet}

<div class="rules-page">
  <h1 class="rules-heading">Rules</h1>
  <ResultsHeader parts={headerParts} k={0} {busy} />
  {#if advisory}
    <div class="rules-advisory">{advisory}</div>
  {/if}
  <div class="rules-body">
    <RecallSplit open={!!params.sel} onclose={closeSel} autoSaveId="engram-rules-split">
      {#snippet list()}
        {#if recallState}
          <RecallState state={recallState} onfix={onRecallFix} onretry={onRecallRetry} />
        {:else}
          <ResultsList
            memories={rules}
            mode="unranked"
            label="Rules"
            openId={params.sel}
            {loading}
            {busy}
            onopen={toggleOpen}
            onescape={closeSel}
            groupKey={(m) => m.scope}
            groupHeader={scopeGroupHeader}
            rowTrailing={sharedChip}
          />
        {/if}
      {/snippet}
      {#snippet detail()}
        <DetailPane
          memory={selectedMemory}
          loading={detailQ.isLoading}
          error={detailQ.error}
          requestedId={params.sel}
          {inResults}
          onclose={closeSel}
          onselect={(id) => navigate({ sel: id })}
        />
      {/snippet}
    </RecallSplit>
  </div>
</div>

<style>
  .rules-page {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .rules-heading {
    flex: none;
    padding: calc(8 * var(--u)) calc(14 * var(--u)) calc(6 * var(--u));
    font-size: calc(16 * var(--u));
    font-weight: 600;
  }
  .rules-advisory {
    flex: none;
    padding: calc(2 * var(--u)) calc(14 * var(--u)) calc(6 * var(--u));
    font-size: calc(11 * var(--u));
    color: var(--text-faint, var(--muted-foreground));
  }
  .rules-body {
    flex: 1;
    min-height: 0;
  }
  .rules-group-key {
    font-family: var(--font-mono, monospace);
    font-weight: 500;
  }
  .rules-group-count {
    color: var(--text-faint, var(--muted-foreground));
  }
</style>
