import { render } from 'vitest-browser-svelte';
import { describe, it, expect } from 'vitest';
import AppShell from './AppShell.svelte';

describe('AppShell', () => {
  it('renders nav links and the command trigger', async () => {
    const screen = await render(AppShell);
    await expect.element(screen.getByRole('link', { name: /observe/i })).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: /search/i })).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: /toggle theme/i })).toBeInTheDocument();
  });

  it('renders the engram brand mark in the header', async () => {
    const screen = await render(AppShell);
    await expect.element(screen.getByRole('img', { name: 'engram' })).toBeInTheDocument();
  });
});
