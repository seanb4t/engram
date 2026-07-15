import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ConnectError, Code } from '@connectrpc/connect';
import { RESUME_KEY } from '$lib/resume';
import DiscoveryFormSheet from './DiscoveryFormSheet.svelte';

const { createMutateSpy, redirectToLoginSpy } = vi.hoisted(() => ({
  createMutateSpy: vi.fn(),
  redirectToLoginSpy: vi.fn()
}));

// Mocks the createMutation hook directly (RESEARCH-sanctioned "mocked hook"
// pattern, mirrors MemoryFormSheet.browser.test.ts) -- the composite/RPC
// shape itself is already covered by mutations/discovery.test.ts (Plan 04).
vi.mock('$lib/mutations/discovery', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/mutations/discovery')>();
  return { ...actual, useCreateDiscovery: () => ({ mutate: createMutateSpy }) };
});

vi.mock('$lib/resume', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/resume')>();
  return { ...actual, redirectToLogin: redirectToLoginSpy };
});

beforeEach(() => {
  createMutateSpy.mockReset();
  redirectToLoginSpy.mockReset();
  sessionStorage.clear();
});

describe('DiscoveryFormSheet — field set', () => {
  it('renders content/kind/citations/summary/tags/scope/visibility, titled New discovery / Create', async () => {
    const screen = await render(DiscoveryFormSheet, { open: true, scope: 'discovery:repo:x' });
    await expect.element(screen.getByText('New discovery')).toBeInTheDocument();
    await expect.element(screen.getByLabelText('content')).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'kind', exact: true })).toHaveTextContent('map');
    await expect.element(screen.getByTestId('citation-row')).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'citation 1 kind' })).toBeInTheDocument();
    await expect.element(screen.getByLabelText('citation 1 ref')).toBeInTheDocument();
    await expect.element(screen.getByLabelText('summary (optional)')).toBeInTheDocument();
    await expect.element(screen.getByLabelText('add tag')).toBeInTheDocument();
    await expect.element(screen.getByRole('textbox', { name: 'scope' })).toHaveValue('discovery:repo:x');
    await expect.element(screen.getByRole('button', { name: 'visibility' })).toHaveTextContent('private');
    await expect.element(screen.getByRole('button', { name: 'Create' })).toBeInTheDocument();
  });

  it('has no edit-mode/prefill-existing prop -- always renders New discovery / Create', async () => {
    const screen = await render(DiscoveryFormSheet, { open: true, scope: 'discovery:repo:x' });
    await expect.element(screen.getByText('New discovery')).toBeInTheDocument();
    await expect.element(screen.getByText('Edit')).not.toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Save' })).not.toBeInTheDocument();
  });
});

describe('DiscoveryFormSheet — client fail-fast', () => {
  it('blocks submit on empty content', async () => {
    const screen = await render(DiscoveryFormSheet, { open: true, scope: 'discovery:repo:x' });
    await screen.getByLabelText('citation 1 ref').fill('README.md');
    await expect.element(screen.getByText('content is required')).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Create' })).toBeDisabled();
    expect(createMutateSpy).not.toHaveBeenCalled();
  });

  it('blocks submit when the sole citation is missing a ref', async () => {
    const screen = await render(DiscoveryFormSheet, { open: true, scope: 'discovery:repo:x' });
    await screen.getByLabelText('content').fill('a real discovery');
    await expect.element(screen.getByText('every citation needs a ref')).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Create' })).toBeDisabled();
  });

  it('blocks submit on a non-discovery: scope', async () => {
    const screen = await render(DiscoveryFormSheet, { open: true, scope: 'repo:x' });
    await screen.getByLabelText('content').fill('a real discovery');
    await screen.getByLabelText('citation 1 ref').fill('README.md');
    await expect.element(screen.getByText('scope must start with discovery:')).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Create' })).toBeDisabled();
    expect(createMutateSpy).not.toHaveBeenCalled();
  });
});

describe('DiscoveryFormSheet — submit', () => {
  it('calls useCreateDiscovery with non-empty content, typed citations, and a discovery:-prefixed scope', async () => {
    const screen = await render(DiscoveryFormSheet, { open: true, scope: 'discovery:repo:x' });
    await screen.getByLabelText('content').fill('a mapped subsystem');
    await screen.getByRole('button', { name: 'citation 1 kind' }).click();
    await screen.getByRole('option', { name: 'url' }).click();
    await screen.getByLabelText('citation 1 ref').fill('https://example.com/doc');
    await screen.getByRole('button', { name: 'Create' }).click();

    expect(createMutateSpy).toHaveBeenCalledTimes(1);
    const [vars] = createMutateSpy.mock.calls[0];
    expect(vars.content).toBe('a mapped subsystem');
    expect(vars.scope).toBe('discovery:repo:x');
    expect(vars.citations).toEqual([{ kind: 'url', ref: 'https://example.com/doc' }]);
    expect(vars.shared).toBe(false);
  });

  it('supports adding a second citation row', async () => {
    const screen = await render(DiscoveryFormSheet, { open: true, scope: 'discovery:repo:x' });
    await screen.getByLabelText('content').fill('a mapped subsystem');
    await screen.getByLabelText('citation 1 ref').fill('README.md');
    await screen.getByRole('button', { name: 'add citation' }).click();
    await expect.element(screen.getByLabelText('citation 2 ref')).toBeInTheDocument();
    await screen.getByLabelText('citation 2 ref').fill('a1b2c3d');
    await screen.getByRole('button', { name: 'citation 2 kind' }).click();
    await screen.getByRole('option', { name: 'commit' }).click();
    await screen.getByRole('button', { name: 'Create' }).click();

    const [vars] = createMutateSpy.mock.calls[0];
    expect(vars.citations).toEqual([
      { kind: 'file', ref: 'README.md' },
      { kind: 'commit', ref: 'a1b2c3d' }
    ]);
  });
});

describe('DiscoveryFormSheet — created_private composite result', () => {
  it('closes the sheet as SUCCESS without persisting a resume envelope or entering the re-auth tier', async () => {
    createMutateSpy.mockImplementation((_vars, opts) => {
      opts.onSuccess({ status: 'created_private', id: 'd9' });
    });
    const screen = await render(DiscoveryFormSheet, { open: true, scope: 'discovery:repo:x' });
    await screen.getByLabelText('content').fill('a mapped subsystem');
    await screen.getByLabelText('citation 1 ref').fill('README.md');
    await screen.getByRole('button', { name: 'Create' }).click();
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
    expect(sessionStorage.getItem(RESUME_KEY)).toBeNull();
  });
});

describe('DiscoveryFormSheet — D-09 inline re-auth + resume', () => {
  it('keeps the sheet open with values intact on a post-retry Unauthenticated failure', async () => {
    createMutateSpy.mockImplementation((_vars, opts) => {
      opts.onError(new ConnectError('session expired', Code.Unauthenticated));
    });
    const screen = await render(DiscoveryFormSheet, { open: true, scope: 'discovery:repo:x' });
    await screen.getByLabelText('content').fill('at-risk discovery content');
    await screen.getByLabelText('citation 1 ref').fill('README.md');
    await screen.getByRole('button', { name: 'Create' }).click();

    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
    await expect
      .element(screen.getByText('write failed — session expired. re-authenticate to continue.'))
      .toBeInTheDocument();
    await expect.element(screen.getByLabelText('content')).toHaveValue('at-risk discovery content');
  });

  it('Re-authenticate persists a resume envelope (kind: discovery, mode: create) before navigating', async () => {
    createMutateSpy.mockImplementation((_vars, opts) => {
      opts.onError(new ConnectError('session expired', Code.Unauthenticated));
    });
    const screen = await render(DiscoveryFormSheet, { open: true, scope: 'discovery:repo:x' });
    await screen.getByLabelText('content').fill('at-risk discovery content');
    await screen.getByLabelText('citation 1 ref').fill('README.md');
    await screen.getByRole('button', { name: 'Create' }).click();
    await screen.getByRole('button', { name: 'Re-authenticate' }).click();

    expect(redirectToLoginSpy).toHaveBeenCalledTimes(1);
    const raw = sessionStorage.getItem(RESUME_KEY);
    expect(raw).not.toBeNull();
    const envelope = JSON.parse(raw as string);
    expect(envelope.kind).toBe('discovery');
    expect(envelope.mode).toBe('create');
    expect(envelope.recordId).toBeNull();
    expect(envelope.returnPath.startsWith('/ui')).toBe(false);
    expect(envelope.values.content).toBe('at-risk discovery content');
  });

  it('applies resumeValues PROPS to $state and fires onresumeapplied, without touching sessionStorage itself', async () => {
    const onresumeapplied = vi.fn();
    sessionStorage.setItem(RESUME_KEY, 'untouched-marker');
    const screen = await render(DiscoveryFormSheet, {
      open: true,
      scope: 'discovery:repo:x',
      resumeValues: {
        content: 'restored discovery content',
        kind: 'fact',
        scope: 'discovery:repo:restored',
        citations: [{ kind: 'url', ref: 'https://example.com' }]
      },
      onresumeapplied
    });
    await expect.element(screen.getByLabelText('content')).toHaveValue('restored discovery content');
    await expect.element(screen.getByRole('button', { name: 'kind', exact: true })).toHaveTextContent('fact');
    await expect.element(screen.getByRole('textbox', { name: 'scope' })).toHaveValue('discovery:repo:restored');
    await expect.element(screen.getByLabelText('citation 1 ref')).toHaveValue('https://example.com');
    expect(onresumeapplied).toHaveBeenCalledTimes(1);
    expect(sessionStorage.getItem(RESUME_KEY)).toBe('untouched-marker');
  });
});
