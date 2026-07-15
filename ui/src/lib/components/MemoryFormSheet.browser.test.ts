import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { ConnectError, Code } from '@connectrpc/connect';
import { MemorySchema, type Memory } from '$lib/gen/engram_pb';
import { RESUME_KEY } from '$lib/resume';
import MemoryFormSheet from './MemoryFormSheet.svelte';

const { createMutateSpy, updateMutateSpy, scheduleMutateSpy, redirectToLoginSpy } = vi.hoisted(() => ({
  createMutateSpy: vi.fn(),
  updateMutateSpy: vi.fn(),
  scheduleMutateSpy: vi.fn(),
  redirectToLoginSpy: vi.fn()
}));

// Mocks the createMutation hooks directly (RESEARCH-sanctioned "mocked hook"
// testing pattern) -- avoids needing a QueryClientProvider/engramWrite
// transport double just to exercise the form's own field/validation/D-09
// logic, which is what this suite targets (the mutation internals themselves
// are already covered by mutations/memory.test.ts, Plan 04).
vi.mock('$lib/mutations/memory', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/mutations/memory')>();
  return {
    ...actual,
    useCreateMemory: () => ({ mutate: createMutateSpy }),
    useUpdateMemory: () => ({ mutate: updateMutateSpy }),
    useScheduleMemory: () => ({ mutate: scheduleMutateSpy })
  };
});

// Only redirectToLogin is stubbed (real browser navigation would otherwise
// tear down the running test page) -- persistResume/peekResume/consumeResume
// stay real so the sessionStorage assertions below exercise the actual
// resume.ts module (covered independently by resume.test.ts).
vi.mock('$lib/resume', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/resume')>();
  return { ...actual, redirectToLogin: redirectToLoginSpy };
});

function fakeMemory(overrides: Partial<{
  id: string;
  content: string;
  scope: string;
  category: string;
  tags: string[];
  summary: string;
  visibility: string;
}> = {}): Memory {
  return create(MemorySchema, {
    id: 'm1',
    content: 'original content',
    scope: 'repo:x',
    category: 'gotcha',
    tags: ['a', 'b'],
    summary: 'orig summary',
    visibility: 'private',
    ...overrides
  });
}

beforeEach(() => {
  createMutateSpy.mockReset();
  updateMutateSpy.mockReset();
  scheduleMutateSpy.mockReset();
  redirectToLoginSpy.mockReset();
  sessionStorage.clear();
});

describe('MemoryFormSheet — create mode field set', () => {
  it('renders content/scope/category/tags/visibility/summary and the schedule toggle, titled New memory / Create', async () => {
    const screen = await render(MemoryFormSheet, { open: true, mode: 'create', scope: 'repo:default' });
    await expect.element(screen.getByText('New memory')).toBeInTheDocument();
    await expect.element(screen.getByLabelText('content')).toBeInTheDocument();
    await expect.element(screen.getByLabelText('content')).toHaveAttribute('placeholder', 'write the memory…');
    await expect.element(screen.getByRole('button', { name: 'category' })).toBeInTheDocument();
    await expect.element(screen.getByLabelText('add tag')).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'visibility' })).toBeInTheDocument();
    await expect.element(screen.getByLabelText('summary (optional)')).toBeInTheDocument();
    await expect.element(screen.getByLabelText('schedule this memory')).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Create' })).toBeInTheDocument();
  });

  it('defaults the scope input to the passed current scope', async () => {
    const screen = await render(MemoryFormSheet, { open: true, mode: 'create', scope: 'repo:default' });
    await expect.element(screen.getByRole('textbox', { name: 'scope' })).toHaveValue('repo:default');
  });

  it('blocks submit on a blank/whitespace scope with a field error', async () => {
    const screen = await render(MemoryFormSheet, { open: true, mode: 'create', scope: '' });
    await screen.getByLabelText('content').fill('some content');
    await expect.element(screen.getByText('scope is required')).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Create' })).toBeDisabled();
    expect(createMutateSpy).not.toHaveBeenCalled();
  });

  it('calls useCreateMemory with the entered values and no shared intent by default', async () => {
    const screen = await render(MemoryFormSheet, { open: true, mode: 'create', scope: 'repo:default' });
    await screen.getByLabelText('content').fill('new memory body');
    await screen.getByRole('button', { name: 'Create' }).click();
    expect(createMutateSpy).toHaveBeenCalledTimes(1);
    const [vars] = createMutateSpy.mock.calls[0];
    expect(vars.content).toBe('new memory body');
    expect(vars.scope).toBe('repo:default');
    expect(vars.shared).toBe(false);
    expect(scheduleMutateSpy).not.toHaveBeenCalled();
  });
});

describe('MemoryFormSheet — share-warning gate', () => {
  it('shows ShareWarningInline on selecting shared and does not set the shared intent until acknowledged', async () => {
    const screen = await render(MemoryFormSheet, { open: true, mode: 'create', scope: 'repo:default' });
    await screen.getByRole('button', { name: 'visibility' }).click();
    await screen.getByRole('option', { name: 'shared' }).click();
    await expect.element(screen.getByText(/sharing makes this readable/)).toBeInTheDocument();

    await screen.getByLabelText('content').fill('shared memory');
    await screen.getByRole('button', { name: 'Create' }).click();
    expect(createMutateSpy).toHaveBeenCalledTimes(1);
    expect(createMutateSpy.mock.calls[0][0].shared).toBe(false);
  });

  it('sets the shared intent to true only after Share anyway is clicked', async () => {
    const screen = await render(MemoryFormSheet, { open: true, mode: 'create', scope: 'repo:default' });
    await screen.getByRole('button', { name: 'visibility' }).click();
    await screen.getByRole('option', { name: 'shared' }).click();
    await screen.getByRole('button', { name: 'Share anyway' }).click();
    await expect.element(screen.getByText(/sharing makes this readable/)).not.toBeInTheDocument();

    await screen.getByLabelText('content').fill('shared memory');
    await screen.getByRole('button', { name: 'Create' }).click();
    expect(createMutateSpy.mock.calls[0][0].shared).toBe(true);
  });

  it('Cancel on the warning reverts the selection to private', async () => {
    const screen = await render(MemoryFormSheet, { open: true, mode: 'create', scope: 'repo:default' });
    await screen.getByRole('button', { name: 'visibility' }).click();
    await screen.getByRole('option', { name: 'shared' }).click();
    await screen.getByRole('alert').getByRole('button', { name: 'Cancel' }).click();
    await expect.element(screen.getByText(/sharing makes this readable/)).not.toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'visibility' })).toHaveTextContent('private');
  });
});

describe('MemoryFormSheet — create+schedule window', () => {
  it('routes through useScheduleMemory (not useCreateMemory) WITH the shared intent when a window + shared are set', async () => {
    const screen = await render(MemoryFormSheet, { open: true, mode: 'create', scope: 'repo:default' });
    await screen.getByLabelText('content').fill('scheduled memory');
    await screen.getByLabelText('schedule this memory').click();
    await screen.getByLabelText('not before').fill('2026-08-01T00:00');

    await screen.getByRole('button', { name: 'visibility' }).click();
    await screen.getByRole('option', { name: 'shared' }).click();
    await screen.getByRole('button', { name: 'Share anyway' }).click();

    await screen.getByRole('button', { name: 'Create' }).click();
    expect(scheduleMutateSpy).toHaveBeenCalledTimes(1);
    expect(createMutateSpy).not.toHaveBeenCalled();
    const [vars] = scheduleMutateSpy.mock.calls[0];
    expect(vars.shared).toBe(true);
    expect(vars.notBefore).toBeInstanceOf(Date);
  });

  it('blocks submit when the schedule toggle is on but no window is set', async () => {
    const screen = await render(MemoryFormSheet, { open: true, mode: 'create', scope: 'repo:default' });
    await screen.getByLabelText('content').fill('scheduled memory');
    await screen.getByLabelText('schedule this memory').click();
    await expect.element(screen.getByText('set a not-before or not-after window')).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Create' })).toBeDisabled();
  });
});

describe('MemoryFormSheet — edit mode', () => {
  it('disables scope/category and hides the schedule toggle, titled Edit memory / Save', async () => {
    const screen = await render(MemoryFormSheet, {
      open: true,
      mode: 'edit',
      memory: fakeMemory(),
      scope: 'repo:x'
    });
    await expect.element(screen.getByText('Edit memory')).toBeInTheDocument();
    await expect.element(screen.getByTestId('scope-readonly')).toHaveTextContent('repo:x');
    await expect.element(screen.getByTestId('category-readonly')).toHaveTextContent('gotcha');
    await expect.element(screen.getByLabelText('schedule this memory')).not.toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
  });

  it('normalizes a stored empty-string visibility to private on prefill', async () => {
    const screen = await render(MemoryFormSheet, {
      open: true,
      mode: 'edit',
      memory: fakeMemory({ visibility: '' }),
      scope: 'repo:x'
    });
    await expect.element(screen.getByRole('button', { name: 'visibility' })).toHaveTextContent('private');
  });

  it('disables Save when nothing has changed, and enables it with only the changed field in the mask', async () => {
    const screen = await render(MemoryFormSheet, {
      open: true,
      mode: 'edit',
      memory: fakeMemory(),
      scope: 'repo:x'
    });
    await expect.element(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
    await screen.getByLabelText('content').fill('updated content');
    await expect.element(screen.getByRole('button', { name: 'Save' })).not.toBeDisabled();
    await screen.getByRole('button', { name: 'Save' }).click();
    expect(updateMutateSpy).toHaveBeenCalledTimes(1);
    const [vars] = updateMutateSpy.mock.calls[0];
    expect(vars).toEqual({ id: 'm1', content: 'updated content' });
  });

  it('renders visibility READ-ONLY for an already-shared record and never emits shared:false', async () => {
    const screen = await render(MemoryFormSheet, {
      open: true,
      mode: 'edit',
      memory: fakeMemory({ visibility: 'shared' }),
      scope: 'repo:x'
    });
    await expect.element(screen.getByTestId('visibility-readonly')).toHaveTextContent('shared');
    await expect.element(screen.getByRole('button', { name: 'visibility' })).not.toBeInTheDocument();

    await screen.getByLabelText('content').fill('updated content on a shared record');
    await screen.getByRole('button', { name: 'Save' }).click();
    const [vars] = updateMutateSpy.mock.calls[0];
    expect('shared' in vars).toBe(false);
  });

  it('allows a PRIVATE record to be moved to shared (shared:true, one-way) via the same warning gate', async () => {
    const screen = await render(MemoryFormSheet, {
      open: true,
      mode: 'edit',
      memory: fakeMemory({ visibility: 'private' }),
      scope: 'repo:x'
    });
    await screen.getByRole('button', { name: 'visibility' }).click();
    await screen.getByRole('option', { name: 'shared' }).click();
    await expect.element(screen.getByText(/sharing makes this readable/)).toBeInTheDocument();
    await screen.getByRole('button', { name: 'Share anyway' }).click();
    await screen.getByRole('button', { name: 'Save' }).click();
    const [vars] = updateMutateSpy.mock.calls[0];
    expect(vars.shared).toBe(true);
  });
});

describe('MemoryFormSheet — created_private composite result', () => {
  it('closes the sheet as SUCCESS without persisting a resume envelope or entering the re-auth tier', async () => {
    createMutateSpy.mockImplementation((_vars, opts) => {
      opts.onSuccess({ status: 'created_private', id: 'm9' });
    });
    const screen = await render(MemoryFormSheet, { open: true, mode: 'create', scope: 'repo:default' });
    await screen.getByLabelText('content').fill('a private-landed record');
    await screen.getByRole('button', { name: 'Create' }).click();
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
    expect(sessionStorage.getItem(RESUME_KEY)).toBeNull();
  });
});

describe('MemoryFormSheet — D-09 inline re-auth + resume', () => {
  it('keeps the sheet open with values intact on a post-retry Unauthenticated failure', async () => {
    createMutateSpy.mockImplementation((_vars, opts) => {
      opts.onError(new ConnectError('session expired', Code.Unauthenticated));
    });
    const screen = await render(MemoryFormSheet, { open: true, mode: 'create', scope: 'repo:default' });
    await screen.getByLabelText('content').fill('at-risk content');
    await screen.getByLabelText('add tag').click();
    await userEvent.keyboard('mytag{Enter}');
    await screen.getByRole('button', { name: 'Create' }).click();

    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
    await expect
      .element(screen.getByText('write failed — session expired. re-authenticate to continue.'))
      .toBeInTheDocument();
    await expect.element(screen.getByLabelText('content')).toHaveValue('at-risk content');
    await expect.element(screen.getByText(/mytag/)).toBeInTheDocument();
  });

  it('Re-authenticate persists a versioned resume envelope with a base-relative returnPath before navigating', async () => {
    createMutateSpy.mockImplementation((_vars, opts) => {
      opts.onError(new ConnectError('session expired', Code.Unauthenticated));
    });
    const screen = await render(MemoryFormSheet, { open: true, mode: 'create', scope: 'repo:default' });
    await screen.getByLabelText('content').fill('at-risk content');
    await screen.getByRole('button', { name: 'Create' }).click();
    await screen.getByRole('button', { name: 'Re-authenticate' }).click();

    expect(redirectToLoginSpy).toHaveBeenCalledTimes(1);
    const raw = sessionStorage.getItem(RESUME_KEY);
    expect(raw).not.toBeNull();
    const envelope = JSON.parse(raw as string);
    expect(envelope.kind).toBe('memory');
    expect(envelope.mode).toBe('create');
    expect(typeof envelope.v).toBe('number');
    expect(typeof envelope.ts).toBe('number');
    expect(envelope.returnPath.startsWith('/ui')).toBe(false);
    expect(envelope.values.content).toBe('at-risk content');
  });

  it('applies resumeValues PROPS to $state and fires onresumeapplied, without the form touching sessionStorage itself', async () => {
    const onresumeapplied = vi.fn();
    sessionStorage.setItem(RESUME_KEY, 'untouched-marker');
    const screen = await render(MemoryFormSheet, {
      open: true,
      mode: 'create',
      scope: 'repo:default',
      resumeValues: { content: 'restored content', scope: 'repo:restored', tags: ['restored-tag'] },
      onresumeapplied
    });
    await expect.element(screen.getByLabelText('content')).toHaveValue('restored content');
    await expect.element(screen.getByRole('textbox', { name: 'scope' })).toHaveValue('repo:restored');
    await expect.element(screen.getByText(/restored-tag/)).toBeInTheDocument();
    expect(onresumeapplied).toHaveBeenCalledTimes(1);
    // The form never reads/deletes sessionStorage itself -- the marker set
    // above (simulating the host's own storage) is left completely alone.
    expect(sessionStorage.getItem(RESUME_KEY)).toBe('untouched-marker');
  });
});
