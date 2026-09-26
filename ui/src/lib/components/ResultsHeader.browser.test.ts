// See ResultRow.browser.test.ts for why this import is required: isolated
// component mounts never pull in +layout.svelte's app.css, so --u and the
// design tokens are otherwise invalid/no-op in this cascade.
import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { describe, it, expect } from 'vitest';
import { rankedHeaderParts, type HeaderPart, type HiddenCounts } from '$lib/search/recall-header';
import ResultsHeader from './ResultsHeader.svelte';

const zeroHidden: HiddenCounts = { total: 0, archived: 0, superseded: 0, expired: 0, scheduled: 0 };

describe('ResultsHeader', () => {
  it('reveals searched_scopes with each scope\'s hit count "in top k", zeros included, when the scopes button is clicked', async () => {
    const parts = rankedHeaderParts({ hits: 3, scopeCount: 3, query: 'x', reranked: false, hidden: zeroHidden });
    const scopeHits = [
      { scope: 'repo:a', hits: 2, searched: true },
      { scope: 'repo:b', hits: 1, searched: true },
      { scope: 'repo:c', hits: 0, searched: true }
    ];
    const screen = await render(ResultsHeader, { parts, scopeHits, k: 50, busy: false });

    const button = screen.getByRole('button', { name: /across 3 scopes/ });
    await button.click();

    await expect.element(screen.getByText('searched_scopes:')).toBeInTheDocument();
    await expect.element(screen.getByText('repo:a')).toBeInTheDocument();
    await expect.element(screen.getByText('repo:c')).toBeInTheDocument();
    expect(screen.container.querySelectorAll('.rh-scope-note').length).toBe(3);
    expect(screen.container.textContent).toContain('in top 50');
  });

  it('carries the hidden clause tooltip with the per-state breakdown', async () => {
    const hidden: HiddenCounts = { total: 2, archived: 1, superseded: 1, expired: 0, scheduled: 0 };
    const parts = rankedHeaderParts({ hits: 5, scopeCount: 2, query: 'x', reranked: false, hidden });
    const screen = await render(ResultsHeader, { parts, k: 50, busy: false });

    const hiddenEl = screen.container.querySelector('[title="1 archived, 1 superseded"]');
    expect(hiddenEl).toBeTruthy();
    expect(hiddenEl?.textContent).toContain('2 hidden by recall gate');
  });

  it('ellipsizes a long query in the mono code span and keeps the full text in its title', async () => {
    const longQuery = 'q'.repeat(200);
    const parts: HeaderPart[] = [{ kind: 'query', text: `for ${longQuery}`, title: longQuery }];
    const screen = await render(ResultsHeader, { parts, k: 50, busy: false });

    const code = screen.container.querySelector('.rh-query');
    expect(code?.getAttribute('title')).toBe(longQuery);
    expect(code?.textContent).toBe(longQuery);
    const style = code ? getComputedStyle(code) : undefined;
    expect(style?.textOverflow).toBe('ellipsis');
  });
});
