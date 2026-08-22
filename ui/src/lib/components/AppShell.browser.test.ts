import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { MigrateStatusResponseSchema } from '$lib/gen/engram_pb';
import AppShell from './AppShell.svelte';

const { migrateStatusSpy } = vi.hoisted(() => ({ migrateStatusSpy: vi.fn() }));

vi.mock('$lib/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/client')>();
  return { ...actual, engram: { ...actual.engram, migrateStatus: migrateStatusSpy } };
});

let qc: QueryClient;

function renderShell() {
  return render(AppShell, {}, { wrapper: QueryClientProvider, wrapperProps: { client: qc } });
}

beforeEach(() => {
  migrateStatusSpy.mockReset();
  migrateStatusSpy.mockResolvedValue(create(MigrateStatusResponseSchema, { pending: 0n, futureTotal: 0n }));
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

describe('AppShell', () => {
  it('renders nav links and the command trigger', async () => {
    const screen = renderShell();
    await expect.element(screen.getByRole('link', { name: /observe/i })).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: /search/i })).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: /toggle theme/i })).toBeInTheDocument();
  });

  it('renders the engram brand mark in the header', async () => {
    const screen = renderShell();
    await expect.element(screen.getByRole('img', { name: 'engram' })).toBeInTheDocument();
  });

  it('renders no migration strip for a zero/zero response', async () => {
    const screen = renderShell();
    await expect.element(screen.getByRole('link', { name: /observe/i })).toBeInTheDocument();
    await expect.element(screen.getByText(/pending migration/)).not.toBeInTheDocument();
  });

  it('renders a migration strip between the header and the route content row for a non-zero response', async () => {
    migrateStatusSpy.mockResolvedValue(create(MigrateStatusResponseSchema, { pending: 4n, futureTotal: 0n }));
    const screen = renderShell();
    await expect.element(screen.getByText(/pending migration/)).toBeInTheDocument();

    const shell = screen.container.querySelector('.h-dvh')!;
    const children = Array.from(shell.children);
    const headerIndex = children.findIndex((el) => el.tagName === 'HEADER');
    const stripIndex = children.findIndex((el) => el.textContent?.includes('pending migration'));
    const contentRowIndex = children.findIndex((el) => el.classList.contains('flex-1') && el.classList.contains('min-h-0'));
    expect(headerIndex).toBeGreaterThanOrEqual(0);
    expect(stripIndex).toBeGreaterThan(headerIndex);
    expect(contentRowIndex).toBeGreaterThan(stripIndex);
  });
});
