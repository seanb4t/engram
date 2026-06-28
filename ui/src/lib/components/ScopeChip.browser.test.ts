import { render } from 'vitest-browser-svelte';
import { describe, it, expect } from 'vitest';
import ScopeChip from './ScopeChip.svelte';

describe('ScopeChip', () => {
  it('shows repo name prominently and the type badge', async () => {
    const screen = await render(ScopeChip, { scope: 'repo:github.com/fzymgc-house/selfhosted-cluster' });
    await expect.element(screen.getByText('selfhosted-cluster')).toBeInTheDocument();
    await expect.element(screen.getByText('repo')).toBeInTheDocument();
  });
  it('keeps the full scope available (title attr) — never destroyed', async () => {
    const screen = await render(ScopeChip, { scope: 'repo:github.com/fzymgc-house/selfhosted-cluster' });
    await expect.element(screen.getByTitle('repo:github.com/fzymgc-house/selfhosted-cluster')).toBeInTheDocument();
  });
});
