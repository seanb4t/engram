import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import AppShell from './AppShell.svelte';

describe('AppShell', () => {
  it('renders nav links and the command trigger', () => {
    render(AppShell);
    expect(screen.getByRole('link', { name: /observe/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /search/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /toggle theme/i })).toBeInTheDocument();
  });
});
