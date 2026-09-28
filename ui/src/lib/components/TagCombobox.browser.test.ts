// See ResultRow.browser.test.ts for why this import is required: isolated
// component mounts never pull in +layout.svelte's app.css, so --u and the
// design tokens are otherwise invalid/no-op in this cascade.
import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { page, userEvent } from 'vitest/browser';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { ConnectError, Code } from '@connectrpc/connect';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { ListTagsResponseSchema } from '$lib/gen/engram_pb';
import TagCombobox from './TagCombobox.svelte';

const { listTagsSpy } = vi.hoisted(() => ({ listTagsSpy: vi.fn() }));

vi.mock('$lib/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/client')>();
  return { ...actual, engram: { ...actual.engram, listTags: listTagsSpy } };
});

function fakeTags(more = false) {
  return create(ListTagsResponseSchema, {
    tags: [
      { tag: 'qdrant', count: 9n },
      { tag: 'sqlite', count: 12n },
      { tag: 'qdr-ops', count: 2n },
      { tag: 'quad', count: 9n }
    ],
    more
  });
}

let qc: QueryClient;

function renderCombobox(props: Record<string, unknown> = {}, client: QueryClient = qc) {
  return render(
    TagCombobox,
    { scope: 'repo:acme/x', onadd: vi.fn(), ...props },
    { wrapper: QueryClientProvider, wrapperProps: { client } }
  );
}

beforeEach(() => {
  listTagsSpy.mockReset();
  listTagsSpy.mockResolvedValue(fakeTags());
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

describe('TagCombobox — opening and typing', () => {
  it('the "+ tag" trigger opens a popover whose input has focus', async () => {
    const screen = await renderCombobox();
    await screen.getByRole('button', { name: '+ tag' }).click();

    const input = screen.getByRole('combobox', { name: 'Filter tags' });
    await expect.element(input).toBeInTheDocument();
    await expect.poll(() => document.activeElement === input.element()).toBe(true);
  });

  it('typing "qd" lists the ranked rows with bolded hits, mini bars and counts, the footer and the "Add #qd" row', async () => {
    const screen = await renderCombobox();
    await screen.getByRole('button', { name: '+ tag' }).click();
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);

    const input = screen.getByRole('combobox', { name: 'Filter tags' });
    await input.fill('qd');

    // The popover content is portaled to document.body (CONTEXT.md: "TagCombobox
    // keeps the default Popover portal"), so it lives outside screen.container.
    const rows = document.querySelectorAll('.opt');
    expect(rows.length).toBe(2);
    expect(rows[0].textContent).toContain('qdrant');
    expect(rows[0].querySelector('b')?.textContent).toBe('qd');
    expect(rows[0].querySelector('.mini')).not.toBeNull();
    expect(rows[0].textContent).toContain('9');

    await expect.element(screen.getByText('2 matches · top 8 shown')).toBeInTheDocument();
    await expect.element(screen.getByText('Add #qd')).toBeInTheDocument();
    await expect.element(screen.getByText('#qd — 0 recall-visible records')).toBeInTheDocument();
  });
});

describe('TagCombobox — selection', () => {
  it('Enter on the first ranked row calls onadd with its tag and closes', async () => {
    const onadd = vi.fn();
    const screen = await renderCombobox({ onadd });
    await screen.getByRole('button', { name: '+ tag' }).click();
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);

    const input = screen.getByRole('combobox', { name: 'Filter tags' });
    await input.fill('qd');
    await userEvent.keyboard('{Enter}');

    expect(onadd).toHaveBeenCalledWith('qdrant');
    await expect.element(screen.getByRole('combobox', { name: 'Filter tags' })).not.toBeInTheDocument();
  });

  it('selecting "Add #qd" calls onadd(\'qd\')', async () => {
    const onadd = vi.fn();
    const screen = await renderCombobox({ onadd });
    await screen.getByRole('button', { name: '+ tag' }).click();
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);

    const input = screen.getByRole('combobox', { name: 'Filter tags' });
    await input.fill('qd');
    await screen.getByText('Add #qd').click();

    expect(onadd).toHaveBeenCalledWith('qd');
  });
});

describe('TagCombobox — errors', () => {
  it('a rejected ListTags shows the envelope copy and no match rows', async () => {
    listTagsSpy.mockRejectedValue(
      new ConnectError('field=scope hint=invalid: scope is malformed', Code.FailedPrecondition)
    );
    const screen = await renderCombobox();
    await screen.getByRole('button', { name: '+ tag' }).click();

    await expect.element(screen.getByText('Server rejected the request')).toBeInTheDocument();
    await expect.element(screen.getByText('field=scope hint=invalid: scope is malformed')).toBeInTheDocument();
    expect(document.querySelectorAll('.opt').length).toBe(0);
  });
});

describe('TagCombobox — visual-state backstop (E5 zero-one-many, long-text)', () => {
  it('1-match and 8-match renders produce identical row structure', async () => {
    listTagsSpy.mockResolvedValue(
      create(ListTagsResponseSchema, {
        tags: Array.from({ length: 8 }, (_, i) => ({ tag: `alpha${i}`, count: BigInt(8 - i) })),
        more: false
      })
    );
    const screenMany = await renderCombobox({}, qc);
    await screenMany.getByRole('button', { name: '+ tag' }).click();
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);
    const inputMany = screenMany.getByRole('combobox', { name: 'Filter tags' });
    await inputMany.fill('alpha');
    // The popover content is portaled to document.body -- query there.
    const rowsMany = document.querySelectorAll('.opt');
    expect(rowsMany.length).toBe(8);
    const shapeMany = Array.from(rowsMany).map((r) => r.children.length);
    await screenMany.unmount();

    const qc2 = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    listTagsSpy.mockResolvedValue(
      create(ListTagsResponseSchema, { tags: [{ tag: 'alpha0', count: 8n }], more: false })
    );
    const screenOne = await renderCombobox({}, qc2);
    await screenOne.getByRole('button', { name: '+ tag' }).click();
    const inputOne = screenOne.getByRole('combobox', { name: 'Filter tags' });
    await inputOne.fill('alpha');
    const rowsOne = document.querySelectorAll('.opt');
    expect(rowsOne.length).toBe(1);
    const shapeOne = Array.from(rowsOne).map((r) => r.children.length);

    expect(shapeOne[0]).toBe(shapeMany[0]);
  });

  it('a 128-byte tag row does not overflow its fixed-width name column', async () => {
    const longTag = 'x'.repeat(128);
    listTagsSpy.mockResolvedValue(create(ListTagsResponseSchema, { tags: [{ tag: longTag, count: 5n }], more: false }));
    const screen = await renderCombobox();
    await screen.getByRole('button', { name: '+ tag' }).click();
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);
    const input = screen.getByRole('combobox', { name: 'Filter tags' });
    await input.fill('x');

    const nameEl = document.querySelector('.opt .nm') as HTMLElement;
    expect(nameEl).not.toBeNull();
    expect(nameEl.scrollWidth).toBeGreaterThanOrEqual(nameEl.clientWidth);
    // scrollWidth must not exceed the visual row width -- ellipsis contains it.
    const optEl = document.querySelector('.opt') as HTMLElement;
    expect(nameEl.getBoundingClientRect().width).toBeLessThanOrEqual(optEl.getBoundingClientRect().width);
  });
});

describe('TagCombobox — screenshots (light + dark)', () => {
  afterEach(() => {
    document.documentElement.classList.remove('dark');
  });

  it('captures the populated picker in both themes', async () => {
    const screen = await renderCombobox();
    await screen.getByRole('button', { name: '+ tag' }).click();
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);
    const input = screen.getByRole('combobox', { name: 'Filter tags' });
    await input.fill('qd');

    document.documentElement.classList.remove('dark');
    await page.screenshot();

    document.documentElement.classList.add('dark');
    await page.screenshot();
  });
});
