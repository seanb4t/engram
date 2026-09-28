import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { ConnectError, Code } from '@connectrpc/connect';
import { create, type MessageInitShape } from '@bufbuild/protobuf';
import {
  MemorySchema,
  RelatedMemoriesResponseSchema,
  RelatedMemorySchema,
  RelatedEdgeSchema,
  VectorEvidenceSchema,
  TagEvidenceSchema,
  SupersessionEvidenceSchema,
  WeightedTagSchema,
  EdgeType,
  SupersessionDirection,
  type Memory
} from '$lib/gen/engram_pb';
import RelatedPage from './[id]/+page.svelte';

// vi.hoisted runs before this module's own imports are linked, so a dynamic
// import() inside the (awaited) factory is used for SvelteURL -- mirrors
// scheduled.browser.test.ts's identical ordering constraint. pageState.params
// is a getter parsed from the reactive url.pathname (no SvelteKit router in
// this test harness, unlike the flat-searchParams routes).
const { gotoSpy, pageState, relatedMemoriesSpy } = await vi.hoisted(async () => {
  const { SvelteURL } = await import('svelte/reactivity');
  const url = new SvelteURL('http://localhost/related/anchor-uuid');
  const pageState = {
    url,
    get params() {
      const m = /\/related\/([^/]+)/.exec(url.pathname);
      return { id: m ? m[1] : '' };
    }
  };
  const gotoSpy = vi.fn((href: string) => {
    const next = new URL(href, 'http://localhost');
    pageState.url.href = next.href;
  });
  return { gotoSpy, pageState, relatedMemoriesSpy: vi.fn() };
});

vi.mock('$app/navigation', () => ({ goto: gotoSpy }));
vi.mock('$app/paths', () => ({ base: '/ui' }));
vi.mock('$app/state', () => ({ page: pageState }));

vi.mock('$lib/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/client')>();
  return {
    ...actual,
    engram: { ...actual.engram, relatedMemories: relatedMemoriesSpy }
  };
});

let qc: QueryClient;
function renderRelated() {
  return render(RelatedPage, {}, { wrapper: QueryClientProvider, wrapperProps: { client: qc } });
}

function makeMemory(overrides: MessageInitShape<typeof MemorySchema> = {}): Memory {
  return create(MemorySchema, {
    id: 'm-default',
    category: 'convention',
    summary: 'default summary',
    content: '',
    tags: [],
    scope: 'repo:test',
    visibility: 'private',
    owner: 'me',
    shortId: 's0000000001',
    ...overrides
  });
}

// Fixture: an anchor plus three candidates -- one supersession predecessor,
// one tag-only, one tag+vector (a multi-type candidate).
function fixtureResponse() {
  const anchor = makeMemory({ id: 'anchor-uuid', shortId: 'anchor00001', category: 'decision', summary: 'Anchor summary', tags: ['engram'] });

  const predecessor = makeMemory({ id: 'pred-uuid', shortId: 'pred0000001', category: 'gotcha', summary: 'Predecessor summary' });
  const predEdge = create(RelatedEdgeSchema, {
    type: EdgeType.SUPERSESSION,
    evidence: { case: 'supersession', value: create(SupersessionEvidenceSchema, { direction: SupersessionDirection.PREDECESSOR, depth: 1 }) }
  });

  const tagCand = makeMemory({ id: 'tag-uuid', shortId: 'tagc0000001', category: 'preference', summary: 'Tag candidate' });
  const tagEdge = create(RelatedEdgeSchema, {
    type: EdgeType.TAG,
    evidence: {
      case: 'tag',
      value: create(TagEvidenceSchema, { sharedTags: [create(WeightedTagSchema, { tag: 'engram', weight: 2.1 })], tagWeight: 2.1 })
    }
  });

  const tagVecCand = makeMemory({ id: 'tagvec-uuid', shortId: 'tagv0000001', category: 'discovery', summary: 'Tag and vector candidate' });
  const tagEdge2 = create(RelatedEdgeSchema, {
    type: EdgeType.TAG,
    evidence: {
      case: 'tag',
      value: create(TagEvidenceSchema, { sharedTags: [create(WeightedTagSchema, { tag: 'engram', weight: 1.4 })], tagWeight: 1.4 })
    }
  });
  const vectorEdge = create(RelatedEdgeSchema, {
    type: EdgeType.VECTOR,
    evidence: { case: 'vector', value: create(VectorEvidenceSchema, { score: 0.81 }) }
  });

  return create(RelatedMemoriesResponseSchema, {
    anchor,
    truncated: false,
    related: [
      create(RelatedMemorySchema, { memory: predecessor, edges: [predEdge] }),
      create(RelatedMemorySchema, { memory: tagCand, edges: [tagEdge] }),
      create(RelatedMemorySchema, { memory: tagVecCand, edges: [tagEdge2, vectorEdge] })
    ]
  });
}

beforeEach(() => {
  gotoSpy.mockClear();
  relatedMemoriesSpy.mockReset().mockResolvedValue(fixtureResponse());
  pageState.url.href = 'http://localhost/related/anchor-uuid';
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

describe('/related/[id] — fetches one RelatedMemories response and draws the neighbourhood (Task 1 tracer)', () => {
  it('calls RelatedMemories once with { id, k: 64n, full: false } through the read client', async () => {
    renderRelated();
    await expect.poll(() => relatedMemoriesSpy.mock.calls.length).toBe(1);
    expect(relatedMemoriesSpy).toHaveBeenCalledWith(
      { id: 'anchor-uuid', k: 64n, full: false },
      expect.objectContaining({ signal: expect.anything() })
    );
  });

  it('renders the mono call line with per-type counts and truncated=false', async () => {
    const screen = await renderRelated();
    await expect
      .element(screen.getByText(/RelatedMemories\(subj, "anchor00001", k=64\)/))
      .toBeInTheDocument();
    await expect.poll(() => screen.container.textContent ?? '').toContain('3 related');
    await expect.poll(() => screen.container.textContent ?? '').toContain('supersession 1');
    await expect.poll(() => screen.container.textContent ?? '').toContain('citation 0');
    await expect.poll(() => screen.container.textContent ?? '').toContain('tag 2');
    await expect.poll(() => screen.container.textContent ?? '').toContain('vector 1');
    await expect.poll(() => screen.container.textContent ?? '').toContain('truncated=false');
  });

  it('draws exactly four role=option nodes: the anchor plus its three candidates', async () => {
    const screen = await renderRelated();
    await expect.poll(() => screen.container.querySelectorAll('[role="option"]').length).toBe(4);
    expect(screen.container.querySelector('#gn-anchor-uuid')).not.toBeNull();
    expect(screen.container.querySelector('#gn-pred-uuid')).not.toBeNull();
    expect(screen.container.querySelector('#gn-tag-uuid')).not.toBeNull();
    expect(screen.container.querySelector('#gn-tagvec-uuid')).not.toBeNull();
  });
});

describe('/related/[id] — lanes, shared selection and evidence under the graph (Task 1)', () => {
  it('clicking a tag-lane row lights every appearance of that candidate and opens the evidence section under the graph', async () => {
    const screen = await renderRelated();
    await expect.element(screen.getByTestId('lane-row-tag-tagvec-uuid')).toBeInTheDocument();

    await screen.getByTestId('lane-row-tag-tagvec-uuid').click();

    await expect.poll(() => screen.container.querySelector('[data-testid="lane-row-tag-tagvec-uuid"]')?.classList.contains('sel')).toBe(true);
    expect(screen.container.querySelector('[data-testid="lane-row-tag-tagvec-uuid"]')?.classList.contains('sel-here')).toBe(true);

    const vectorRow = screen.container.querySelector('[data-testid="lane-row-vector-tagvec-uuid"]');
    expect(vectorRow?.classList.contains('sel')).toBe(true);
    expect(vectorRow?.classList.contains('sel-here')).toBe(false);

    expect(screen.container.querySelector('#gn-tagvec-uuid')?.classList.contains('sel')).toBe(true);

    const rail = screen.container.querySelector('.rail');
    const railChildren = Array.from(rail?.children ?? []);
    const graphIdx = railChildren.findIndex((el) => el.matches('svg.graph'));
    const evIdx = railChildren.findIndex((el) => el.matches('.ev'));
    expect(graphIdx).toBeGreaterThanOrEqual(0);
    expect(evIdx).toBeGreaterThan(graphIdx);

    await expect.element(screen.getByRole('region', { name: 'Why tagv0000001 is related' })).toBeInTheDocument();
    await expect.poll(() => rail?.querySelector('.ev')?.textContent ?? '').toContain('Why it is related · 2 edge types');
    await expect.poll(() => rail?.querySelector('.ev')?.textContent ?? '').toContain('#engram 1.40');
    await expect.element(screen.getByText('Evidence is per type; there is no blended score.')).toBeInTheDocument();
  });

  it('clicking the node opens the same evidence section', async () => {
    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();
    await expect.poll(() => screen.container.querySelector('#gn-tagvec-uuid')).not.toBeNull();
    screen.container.querySelector('#gn-tagvec-uuid')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await expect.element(screen.getByRole('region', { name: 'Why tagv0000001 is related' })).toBeInTheDocument();
  });

  it('esc × clears the selection and removes the evidence section', async () => {
    const screen = await renderRelated();
    await screen.getByTestId('lane-row-tag-tagvec-uuid').click();
    await expect.element(screen.getByRole('region', { name: 'Why tagv0000001 is related' })).toBeInTheDocument();

    await screen.getByRole('button', { name: 'Clear selection' }).click();
    await expect.poll(() => screen.container.querySelectorAll('.ev').length).toBe(0);
  });

  it('Re-centre navigates to the selected candidate, carrying the anchor onto the trail', async () => {
    const screen = await renderRelated();
    await screen.getByTestId('lane-row-tag-tagvec-uuid').click();
    await screen.getByRole('button', { name: 'Re-centre ↵' }).click();
    expect(gotoSpy).toHaveBeenCalledWith('/ui/related/tagv0000001?trail=anchor00001');
  });

  it('clicking the anchor node selects nothing', async () => {
    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();
    await expect.poll(() => screen.container.querySelector('#gn-anchor-uuid')).not.toBeNull();
    screen.container.querySelector('#gn-anchor-uuid')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(screen.container.querySelectorAll('.ev').length).toBe(0);
  });
});

function zeroCandidateResponse(truncated = false) {
  const anchor = makeMemory({ id: 'anchor-uuid', shortId: 'anchor00001', category: 'decision', summary: 'Anchor summary' });
  return create(RelatedMemoriesResponseSchema, { anchor, truncated, related: [] });
}

describe('/related/[id] — route shell states (Task 3)', () => {
  it('shows the loading copy and four skeleton lane placeholders while RelatedMemories is pending', async () => {
    let resolveIt!: (v: unknown) => void;
    relatedMemoriesSpy.mockReset().mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveIt = resolve;
        })
    );
    const screen = await renderRelated();
    await expect.element(screen.getByText(/resolving anchor, 4 typed sub-queries in flight/)).toBeInTheDocument();
    expect(screen.container.querySelectorAll('[data-testid="lane-skeleton"]').length).toBe(4);
    expect(screen.container.querySelectorAll('[role="option"]').length).toBe(0);
    resolveIt(fixtureResponse());
  });

  it('renders the not-found copy (and nothing else below) for Code.NotFound', async () => {
    relatedMemoriesSpy.mockReset().mockRejectedValue(new ConnectError('not found', Code.NotFound));
    const screen = await renderRelated();
    await expect.element(screen.getByText('No memory with id anchor-uuid that you can read')).toBeInTheDocument();
    expect(screen.container.querySelectorAll('[role="option"]').length).toBe(0);
  });

  it('renders the rejected envelope for a FailedPrecondition field=/hint= rejection', async () => {
    relatedMemoriesSpy
      .mockReset()
      .mockRejectedValue(new ConnectError('field=id hint=invalid_value: id must be a UUID or short_id', Code.FailedPrecondition));
    const screen = await renderRelated();
    await expect.element(screen.getByText('Server rejected the request')).toBeInTheDocument();
    await expect.poll(() => screen.container.textContent ?? '').toContain('field=id hint=invalid_value: id must be a UUID or short_id');
  });

  it('renders the opaque-failure block for an Internal error, with a Retry that refetches', async () => {
    relatedMemoriesSpy.mockReset().mockRejectedValue(new ConnectError('boom', Code.Internal));
    const screen = await renderRelated();
    await expect
      .element(screen.getByText('RelatedMemories failed — nothing was related. related_memories returned code=Internal'))
      .toBeInTheDocument();
    await expect.element(screen.getByText('Nothing was searched, so this is not an empty neighbourhood')).toBeInTheDocument();

    relatedMemoriesSpy.mockClear().mockResolvedValue(fixtureResponse());
    await screen.getByRole('button', { name: 'Retry' }).click();
    await expect.poll(() => relatedMemoriesSpy.mock.calls.length).toBeGreaterThanOrEqual(1);
  });

  it('renders one role=option (the anchor alone) and "0 related" for a zero-candidate response', async () => {
    relatedMemoriesSpy.mockReset().mockResolvedValue(zeroCandidateResponse());
    const screen = await renderRelated();
    await expect.poll(() => screen.container.querySelectorAll('[role="option"]').length).toBe(1);
    await expect.poll(() => screen.container.textContent ?? '').toContain('0 related');
  });

  it('appends a millisecond figure to the call line after the fetch resolves', async () => {
    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();
    await expect.poll(() => /\d+ms/.test(screen.container.textContent ?? '')).toBe(true);
  });

  it('renders the truncated=true word with the warning class when the response was truncated', async () => {
    relatedMemoriesSpy.mockReset().mockResolvedValue(zeroCandidateResponse(true));
    const screen = await renderRelated();
    await expect.poll(() => screen.container.querySelector('.trunc-on')?.textContent ?? '').toBe('truncated=true');
  });

  it('screenshots the loading state', async () => {
    let resolveIt!: (v: unknown) => void;
    relatedMemoriesSpy.mockReset().mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveIt = resolve;
        })
    );
    const screen = await renderRelated();
    await expect.element(screen.getByText(/resolving anchor, 4 typed sub-queries in flight/)).toBeInTheDocument();
    await page.screenshot();
    resolveIt(fixtureResponse());
  });

  it('screenshots the not-found state', async () => {
    relatedMemoriesSpy.mockReset().mockRejectedValue(new ConnectError('not found', Code.NotFound));
    const screen = await renderRelated();
    await expect.element(screen.getByText('No memory with id anchor-uuid that you can read')).toBeInTheDocument();
    await page.screenshot();
  });

  it('screenshots the populated state', async () => {
    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();
    await page.screenshot();
  });
});
