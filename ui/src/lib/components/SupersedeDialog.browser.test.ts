// See ResultRow.browser.test.ts for why this import is required: isolated
// component mounts never pull in +layout.svelte's app.css, so --u and the
// design tokens are otherwise invalid/no-op in this cascade.
import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi } from 'vitest';
import { page } from 'vitest/browser';
import { create } from '@bufbuild/protobuf';
import { ConnectError, Code } from '@connectrpc/connect';
import { MemorySchema, SupersedeMemoryResponseSchema, type Memory, type SupersedeMemoryResponse } from '$lib/gen/engram_pb';
import type { SupersedeDraft, SupersedeFields } from '$lib/mutations/curation';
import SupersedeDialog from './SupersedeDialog.svelte';

function mem(id: string, overrides: Partial<Record<string, unknown>> = {}): Memory {
  return create(MemorySchema, {
    id,
    shortId: `${id.slice(0, 8).toUpperCase().padEnd(8, '0')}01`,
    summary: `summary for ${id}`,
    content: `content for ${id}`,
    category: 'convention',
    scope: 'repo:test',
    visibility: 'private',
    owner: 'me',
    tags: [],
    ...overrides
  });
}

function fields(overrides: Partial<SupersedeFields> = {}): SupersedeFields {
  return { summary: '', content: 'seed content', category: 'convention', scope: 'repo:test', tags: [], ...overrides };
}

function supersedeResponse(overrides: Partial<Record<string, unknown>> = {}): SupersedeMemoryResponse {
  return create(SupersedeMemoryResponseSchema, {
    id: '',
    shortId: '',
    validated: true,
    supersedes: [],
    targets: [],
    ...overrides
  });
}

const validatedPreview = (): Promise<SupersedeMemoryResponse> => Promise.resolve(supersedeResponse());

async function renderDialog(overrides: Record<string, unknown> = {}) {
  const props = {
    open: true,
    targets: [] as Memory[],
    fields: fields(),
    idempotencyKey: 'idem-1',
    onpreview: vi.fn(validatedPreview),
    onsubmit: vi.fn(async () =>
      create(SupersedeMemoryResponseSchema, { id: 'n1', shortId: 'N1SHORT001', validated: false, supersedes: [], targets: [] })
    ),
    onlookup: vi.fn(),
    onresolvehead: vi.fn(),
    oncancel: vi.fn(),
    ondone: vi.fn(),
    onreauth: vi.fn(),
    ...overrides
  };
  return { screen: await render(SupersedeDialog, props), props };
}

function submitButton(screen: Awaited<ReturnType<typeof render>>, n: number) {
  return screen.getByRole('button', { name: `Supersede ${n} → 1` });
}

describe('SupersedeDialog — client-side per-target issues (instant, E1 partial)', () => {
  it('a rule target shows "rules cannot be superseded — delete it instead" instantly', async () => {
    const { screen } = await renderDialog({ targets: [mem('r1', { category: 'rule' })] });
    await expect.element(screen.getByText('rules cannot be superseded — delete it instead')).toBeInTheDocument();

    // E1 invalid — screenshot 1/4.
    await page.screenshot({ path: '__screenshots__/supersede-dialog-invalid.png' });
  });

  it('a superseded target shows "not the live head" and "use head" swaps the chip', async () => {
    const head = mem('h1', { shortId: 'HEADSHORT01' });
    const onresolvehead = vi.fn().mockResolvedValue(head);
    const { screen } = await renderDialog({ targets: [mem('p1', { supersededBy: 'h1' })], onresolvehead });

    await expect
      .element(screen.getByText(`not the live head — current head is ${head.shortId}`))
      .toBeInTheDocument();

    await screen.getByRole('button', { name: `use head ${head.shortId}` }).click();

    await expect.poll(() => document.querySelectorAll('.sd-chip').length).toBe(1);
    await expect.poll(() => document.querySelector('.sd-chip .sd-sid')?.textContent).toBe(head.shortId);
    await expect.element(screen.getByText('not the live head — current head is HEADSHORT01')).not.toBeInTheDocument();
  });
});

describe('SupersedeDialog — add target by short_id (dedupe)', () => {
  it('adding the same short_id twice yields one chip', async () => {
    const found = mem('m9', { shortId: 'M9SHRT0001' });
    const onlookup = vi.fn().mockResolvedValue(found);
    const { screen } = await renderDialog({ targets: [], onlookup });

    const addInput = screen.getByPlaceholder('add target by short_id…');
    await addInput.fill('M9SHRT0001');
    await expect.element(screen.getByText(/M9SHRT0001 —/)).toBeInTheDocument();
    await screen.getByText(/M9SHRT0001 —/).click();
    await expect.poll(() => document.querySelectorAll('.sd-chip').length).toBe(1);

    await addInput.fill('M9SHRT0001');
    await expect.element(screen.getByText(/M9SHRT0001 —/)).toBeInTheDocument();
    await screen.getByText(/M9SHRT0001 —/).click();

    expect(document.querySelectorAll('.sd-chip').length).toBe(1);
  });
});

describe('SupersedeDialog — gates (E1 partial, zero-one-many)', () => {
  it('with 1 invalid chip the gate reads the Remove copy and the button is disabled', async () => {
    const { screen } = await renderDialog({ targets: [mem('r1', { category: 'rule' }), mem('m2')] });
    await expect
      .element(screen.getByText('Remove 1 invalid target(s) — an invalid set rejects the whole call'))
      .toBeInTheDocument();
    await expect.element(submitButton(screen, 2)).toBeDisabled();
  });

  it('with zero targets the gate reads "Add at least one target"', async () => {
    const { screen } = await renderDialog({ targets: [] });
    await expect.element(screen.getByText('Add at least one target')).toBeInTheDocument();
    await expect.element(submitButton(screen, 0)).toBeDisabled();
  });
});

describe('SupersedeDialog — content required (gate + blur error)', () => {
  it('emptying content shows the gate and the field= envelope error on blur', async () => {
    const { screen } = await renderDialog({ targets: [mem('m1')], fields: fields({ content: '' }) });
    await expect.element(screen.getByText('content is required')).toBeInTheDocument();

    const contentField = screen.getByLabelText('content').element() as HTMLTextAreaElement;
    contentField.focus();
    contentField.blur();

    await expect
      .element(screen.getByText('field=content hint=required: content must not be empty'))
      .toBeInTheDocument();
  });
});

describe('SupersedeDialog — byte counter (E1 populated)', () => {
  it('a 600-byte summary turns the counter danger', async () => {
    const { screen } = await renderDialog({ targets: [mem('m1')] });
    await screen.getByLabelText('summary').fill('a'.repeat(600));
    await expect.poll(() => document.querySelector('.sd-byte-counter-danger')).toBeTruthy();
  });
});

describe('SupersedeDialog — cross-scope warning', () => {
  it('two target scopes show the cross-scope warning', async () => {
    const { screen } = await renderDialog({
      targets: [mem('m1', { scope: 'repo:a' }), mem('m2', { scope: 'repo:b' })]
    });
    await expect.element(screen.getByText('Targets span 2 scopes — the new record lands in one.')).toBeInTheDocument();
  });
});

describe('SupersedeDialog — preview pending keeps the button disabled', () => {
  it('a pending preview keeps the button disabled until it resolves', async () => {
    let resolvePreview!: (v: SupersedeMemoryResponse) => void;
    const gate = new Promise<SupersedeMemoryResponse>((r) => (resolvePreview = r));
    const onpreview = vi.fn().mockReturnValue(gate);
    const { screen } = await renderDialog({ targets: [mem('m1')], onpreview });

    await expect.poll(() => onpreview.mock.calls.length).toBeGreaterThan(0);
    await expect.element(submitButton(screen, 1)).toBeDisabled();

    resolvePreview(supersedeResponse());
    await expect.element(submitButton(screen, 1)).not.toBeDisabled();
  });
});

describe('SupersedeDialog — server rejection on commit (server race, T-04-14)', () => {
  it('a server "target is already superseded" on commit marks the chip and shows the status block', async () => {
    const target = mem('p1');
    const onsubmit = vi
      .fn()
      .mockRejectedValue(new ConnectError('target is already superseded: p1', Code.FailedPrecondition));
    const { screen } = await renderDialog({ targets: [target], onsubmit });

    await expect.element(submitButton(screen, 1)).not.toBeDisabled();
    await submitButton(screen, 1).click();

    await expect.element(screen.getByText('Server rejected the call — nothing was written')).toBeInTheDocument();
    await expect.element(screen.getByText('already superseded by another session')).toBeInTheDocument();

    // E1 re-auth/rejection family — screenshot 2/4 (server rejection state).
    await page.screenshot({ path: '__screenshots__/supersede-dialog-rejected.png' });
  });
});

describe('SupersedeDialog — re-auth on commit', () => {
  it('an Unauthenticated commit shows the re-auth status block and calls onreauth on click', async () => {
    const onsubmit = vi.fn().mockRejectedValue(new ConnectError('session expired', Code.Unauthenticated));
    const onreauth = vi.fn();
    const { screen } = await renderDialog({ targets: [mem('m1')], onsubmit, onreauth });

    await expect.element(submitButton(screen, 1)).not.toBeDisabled();
    await submitButton(screen, 1).click();

    await expect
      .element(screen.getByText('Session expired. Nothing was written; your draft is kept.'))
      .toBeInTheDocument();

    // E1 re-auth — screenshot 3/4.
    await page.screenshot({ path: '__screenshots__/supersede-dialog-reauth.png' });

    await screen.getByRole('button', { name: 'Re-authenticate' }).click();
    expect(onreauth).toHaveBeenCalledTimes(1);
  });
});

describe('SupersedeDialog — Cmd+Enter submits only when enabled', () => {
  it('Cmd+Enter is a no-op while disabled', async () => {
    const onsubmit = vi.fn(async () => ({ id: 'n1', shortId: 'N1SHORT001', validated: false, supersedes: [], targets: [] }));
    const { screen } = await renderDialog({ targets: [], onsubmit });

    const dialog = document.querySelector('[role="dialog"]') as HTMLElement;
    dialog.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', metaKey: true, bubbles: true }));
    expect(onsubmit).not.toHaveBeenCalled();

    await expect.element(submitButton(screen, 0)).toBeDisabled();
  });

  it('Cmd+Enter submits once the button is enabled', async () => {
    const onsubmit = vi.fn(async () => ({ id: 'n1', shortId: 'N1SHORT001', validated: false, supersedes: [], targets: [] }));
    const { screen } = await renderDialog({ targets: [mem('m1')], onsubmit });

    await expect.element(submitButton(screen, 1)).not.toBeDisabled();

    const dialog = document.querySelector('[role="dialog"]') as HTMLElement;
    dialog.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', metaKey: true, bubbles: true }));

    await expect.poll(() => onsubmit.mock.calls.length).toBe(1);
  });
});

describe('SupersedeDialog — focus retained while the preview re-renders', () => {
  it('typing in content keeps focus across the debounced preview re-render', async () => {
    const onpreview = vi.fn(validatedPreview);
    const { screen } = await renderDialog({ targets: [mem('m1')], fields: fields({ content: '' }), onpreview });
    const contentField = screen.getByLabelText('content');

    await contentField.click();
    await contentField.fill('a real correction body');

    // Wait past the debounce so the preview effect actually fires and the
    // chain-preview/status-block region re-renders -- the field must not be
    // remounted (and therefore never loses focus) by that re-render.
    await expect.poll(() => onpreview.mock.calls.length).toBeGreaterThan(0);
    expect(document.activeElement).toBe(contentField.element());
  });
});

describe('SupersedeDialog — success view (E1 zero-one-many)', () => {
  it('commit success shows "No undo." and the screenshot captures the populated + success states', async () => {
    const { screen } = await renderDialog({ targets: [mem('m1')] });

    // E1 populated — screenshot 4/4.
    await page.screenshot({ path: '__screenshots__/supersede-dialog-populated.png' });

    await expect.element(submitButton(screen, 1)).not.toBeDisabled();
    await submitButton(screen, 1).click();
    await expect.element(screen.getByText(/No undo\./)).toBeInTheDocument();
  });
});
