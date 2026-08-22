import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { ConnectError, Code } from '@connectrpc/connect';
import { QueryClient, QueryClientProvider, QueryCache } from '@tanstack/svelte-query';
import { MigrateStatusResponseSchema } from '$lib/gen/engram_pb';
import { handleQueryError, errorBanner } from '$lib/errors';
import { get } from 'svelte/store';
import MigrationBanner from './MigrationBanner.svelte';

const { migrateStatusSpy } = vi.hoisted(() => ({ migrateStatusSpy: vi.fn() }));

// mapAuthError is real (not overridden) -- the non-auth errors this suite
// drives through it resolve to null naturally, so no stub is needed.
vi.mock('$lib/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/client')>();
  return { ...actual, engram: { ...actual.engram, migrateStatus: migrateStatusSpy } };
});

function fakeStatus(overrides: Partial<{ pending: bigint; futureTotal: bigint }> = {}) {
  return create(MigrateStatusResponseSchema, { pending: 0n, futureTotal: 0n, ...overrides });
}

let qc: QueryClient;

function renderBanner(client: QueryClient = qc) {
  return render(MigrationBanner, {}, { wrapper: QueryClientProvider, wrapperProps: { client } });
}

beforeEach(() => {
  migrateStatusSpy.mockReset();
  errorBanner.set(null);
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

describe('MigrationBanner — silent at zero, while loading, and on failure', () => {
  it('renders nothing for a zero/zero response', async () => {
    migrateStatusSpy.mockResolvedValue(fakeStatus());
    const screen = renderBanner();
    await expect.element(screen.getByText(/pending migration/)).not.toBeInTheDocument();
    await expect.element(screen.getByText(/newer schema version/)).not.toBeInTheDocument();
    expect(screen.container.textContent?.trim()).toBe('');
  });

  it('renders nothing while the query is loading', async () => {
    let resolveFetch!: (v: ReturnType<typeof fakeStatus>) => void;
    migrateStatusSpy.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveFetch = resolve;
        })
    );
    const screen = renderBanner();
    expect(screen.container.textContent?.trim()).toBe('');
    resolveFetch(fakeStatus());
    await expect.poll(() => migrateStatusSpy).toHaveBeenCalledTimes(1);
  });

  it('rejecting the migration query, through the production handleQueryError wiring, renders nothing, leaves the global errorBanner null, and logs to console.error exactly once', async () => {
    migrateStatusSpy.mockRejectedValue(new ConnectError('boom', Code.Internal));
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
      queryCache: new QueryCache({ onError: handleQueryError })
    });
    const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    const screen = renderBanner(client);
    await expect.poll(() => migrateStatusSpy).toHaveBeenCalledTimes(1);
    await expect.poll(() => consoleErrorSpy).toHaveBeenCalledTimes(1);
    await expect.element(screen.getByText(/pending migration/)).not.toBeInTheDocument();
    await expect.element(screen.getByText(/newer schema version/)).not.toBeInTheDocument();
    expect(get(errorBanner)).toBeNull();
    consoleErrorSpy.mockRestore();
  });
});

describe('MigrationBanner — behind-version strip', () => {
  it('renders one neutral strip naming the pending count and the remediation command, for pending > 0 / futureTotal = 0', async () => {
    migrateStatusSpy.mockResolvedValue(fakeStatus({ pending: 4n }));
    const screen = renderBanner();
    await expect
      .element(screen.getByText('4 records pending migration — run engram migrate'))
      .toBeInTheDocument();
    await expect.element(screen.getByText(/newer schema version/)).not.toBeInTheDocument();
    expect(screen.container.querySelectorAll('div').length).toBe(1);
  });

  it('reads "record" (singular) for pending = 1', async () => {
    migrateStatusSpy.mockResolvedValue(fakeStatus({ pending: 1n }));
    const screen = renderBanner();
    await expect
      .element(screen.getByText('1 record pending migration — run engram migrate'))
      .toBeInTheDocument();
  });
});

describe('MigrationBanner — ahead-version strip', () => {
  it('renders one destructive-toned strip naming the future-version count, for futureTotal > 0 / pending = 0', async () => {
    migrateStatusSpy.mockResolvedValue(fakeStatus({ futureTotal: 2n }));
    const screen = renderBanner();
    await expect
      .element(
        screen.getByText(
          '2 records at a newer schema version than this server — some data may not render correctly'
        )
      )
      .toBeInTheDocument();
    await expect.element(screen.getByText(/pending migration/)).not.toBeInTheDocument();
    expect(screen.container.querySelectorAll('div').length).toBe(1);
  });
});

describe('MigrationBanner — both non-zero', () => {
  it('renders two strips, behind-version first in DOM order, ahead-version second', async () => {
    migrateStatusSpy.mockResolvedValue(fakeStatus({ pending: 3n, futureTotal: 2n }));
    const screen = renderBanner();
    await expect
      .element(screen.getByText('3 records pending migration — run engram migrate'))
      .toBeInTheDocument();
    await expect
      .element(
        screen.getByText(
          '2 records at a newer schema version than this server — some data may not render correctly'
        )
      )
      .toBeInTheDocument();

    const strips = screen.container.querySelectorAll('div');
    expect(strips.length).toBe(2);
    expect(strips[0].textContent).toContain('pending migration');
    expect(strips[1].textContent).toContain('newer schema version');
  });
});
