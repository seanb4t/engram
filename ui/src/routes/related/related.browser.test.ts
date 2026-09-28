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
  ListTagsResponseSchema,
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
const { gotoSpy, pageState, relatedMemoriesSpy, listTagsSpy } = await vi.hoisted(async () => {
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
  return { gotoSpy, pageState, relatedMemoriesSpy: vi.fn(), listTagsSpy: vi.fn() };
});

vi.mock('$app/navigation', () => ({ goto: gotoSpy }));
vi.mock('$app/paths', () => ({ base: '/ui' }));
vi.mock('$app/state', () => ({ page: pageState }));

vi.mock('$lib/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/client')>();
  return {
    ...actual,
    engram: { ...actual.engram, relatedMemories: relatedMemoriesSpy, listTags: listTagsSpy }
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

function emptyTagsResponse() {
  return create(ListTagsResponseSchema, { tags: [], more: false });
}

beforeEach(() => {
  gotoSpy.mockClear();
  relatedMemoriesSpy.mockReset().mockResolvedValue(fixtureResponse());
  listTagsSpy.mockReset().mockResolvedValue(emptyTagsResponse());
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
    const graphIdx = railChildren.findIndex((el) => el.querySelector('svg.graph') !== null);
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

// Fixture: an anchor with a two-predecessor, one-successor supersession
// chain -- pred1 (depth 1, itself superseded by the anchor -- a genuine
// hidden-state member), pred2 (depth 2), succ1 (depth 1 successor).
function chainFixtureResponse(truncated = false) {
  const anchor = makeMemory({ id: 'anchor-uuid', shortId: 'anchor00001', category: 'decision', summary: 'Anchor summary' });
  const pred1 = makeMemory({
    id: 'pred1-uuid',
    shortId: 'pred1000001',
    category: 'gotcha',
    summary: 'Predecessor one',
    supersededBy: 'anchor-uuid'
  });
  const pred2 = makeMemory({ id: 'pred2-uuid', shortId: 'pred2000001', category: 'gotcha', summary: 'Predecessor two', supersededBy: 'pred1-uuid' });
  const succ1 = makeMemory({ id: 'succ1-uuid', shortId: 'succ1000001', category: 'decision', summary: 'Successor one' });

  const pred1Edge = create(RelatedEdgeSchema, {
    type: EdgeType.SUPERSESSION,
    evidence: { case: 'supersession', value: create(SupersessionEvidenceSchema, { direction: SupersessionDirection.PREDECESSOR, depth: 1 }) }
  });
  const pred2Edge = create(RelatedEdgeSchema, {
    type: EdgeType.SUPERSESSION,
    evidence: { case: 'supersession', value: create(SupersessionEvidenceSchema, { direction: SupersessionDirection.PREDECESSOR, depth: 2 }) }
  });
  const succ1Edge = create(RelatedEdgeSchema, {
    type: EdgeType.SUPERSESSION,
    evidence: { case: 'supersession', value: create(SupersessionEvidenceSchema, { direction: SupersessionDirection.SUCCESSOR, depth: 1 }) }
  });

  return create(RelatedMemoriesResponseSchema, {
    anchor,
    truncated,
    related: [
      create(RelatedMemorySchema, { memory: pred1, edges: [pred1Edge] }),
      create(RelatedMemorySchema, { memory: pred2, edges: [pred2Edge] }),
      create(RelatedMemorySchema, { memory: succ1, edges: [succ1Edge] })
    ]
  });
}

describe('/related/[id] — supersession timeline, truncation and empty states (Task 2)', () => {
  it('renders four columns headed −2, −1, anchor, +1 with the anchor .is-anchor and a superseded member .hidden-state', async () => {
    relatedMemoriesSpy.mockReset().mockResolvedValue(chainFixtureResponse());
    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();

    await expect.poll(() => Array.from(screen.container.querySelectorAll('.ccol-h')).map((el) => el.textContent)).toEqual([
      '−2',
      '−1',
      'anchor',
      '+1'
    ]);

    expect(screen.container.querySelector('[data-testid="chain-card-anchor-uuid"]')?.classList.contains('is-anchor')).toBe(true);
    expect(screen.container.querySelector('[data-testid="chain-card-pred1-uuid"]')?.classList.contains('hidden-state')).toBe(true);
  });

  it('clicking a chain card selects it everywhere', async () => {
    relatedMemoriesSpy.mockReset().mockResolvedValue(chainFixtureResponse());
    const screen = await renderRelated();
    await expect.element(screen.getByTestId('chain-card-succ1-uuid')).toBeInTheDocument();

    await screen.getByTestId('chain-card-succ1-uuid').click();

    await expect
      .poll(() => screen.container.querySelector('[data-testid="chain-card-succ1-uuid"]')?.classList.contains('sel-here'))
      .toBe(true);
    expect(screen.container.querySelector('#gn-succ1-uuid')?.classList.contains('sel')).toBe(true);
    await expect.element(screen.getByRole('region', { name: 'Why succ1000001 is related' })).toBeInTheDocument();
  });

  it('renders the ceiling banner when truncated=true', async () => {
    relatedMemoriesSpy.mockReset().mockResolvedValue(chainFixtureResponse(true));
    const screen = await renderRelated();
    await expect
      .element(screen.getByText('▲ truncated=true — total ceiling 64 reached; only the vector lane is cut.'))
      .toBeInTheDocument();
  });

  it("shows the citation lane's empty reason beside a populated tag lane", async () => {
    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();
    await expect.element(screen.getByText('none — no recall-visible record shares a citation (kind + ref)')).toBeInTheDocument();
    await expect.element(screen.getByTestId('lane-row-tag-tag-uuid')).toBeInTheDocument();
  });

  it('renders the no-neighbours card with an Open-in-search link and no lane cards for a zero-candidate response', async () => {
    relatedMemoriesSpy.mockReset().mockResolvedValue(zeroCandidateResponse());
    const screen = await renderRelated();
    await expect.element(screen.getByText('Nothing related to anchor00001')).toBeInTheDocument();
    await expect.element(screen.getByRole('link', { name: 'Open anchor00001 in search ↗' })).toBeInTheDocument();
    expect(screen.container.querySelectorAll('.lane').length).toBe(0);
  });

  it('screenshots the populated supersession-timeline state', async () => {
    relatedMemoriesSpy.mockReset().mockResolvedValue(chainFixtureResponse());
    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();
    await page.screenshot();
  });

  it('screenshots the no-neighbours state', async () => {
    relatedMemoriesSpy.mockReset().mockResolvedValue(zeroCandidateResponse());
    const screen = await renderRelated();
    await expect.element(screen.getByText('Nothing related to anchor00001')).toBeInTheDocument();
    await page.screenshot();
  });
});

// Fixture: an anchor with n vector-only candidates (no other edge types),
// so the vector lane and the graph's non-anchor nodes are exactly this set.
function vectorHitsResponse(n: number, truncated = false) {
  const anchor = makeMemory({ id: 'anchor-uuid', shortId: 'anchor00001', category: 'decision', summary: 'Anchor summary' });
  const related = Array.from({ length: n }, (_, i) => {
    const mem = makeMemory({
      id: `v${i}-uuid`,
      shortId: `vhit${String(i).padStart(2, '0')}0001`,
      category: 'discovery',
      summary: `Vector candidate ${i}`
    });
    const edge = create(RelatedEdgeSchema, {
      type: EdgeType.VECTOR,
      evidence: { case: 'vector', value: create(VectorEvidenceSchema, { score: 0.9 - i * 0.01 }) }
    });
    return create(RelatedMemorySchema, { memory: mem, edges: [edge] });
  });
  return create(RelatedMemoriesResponseSchema, { anchor, truncated, related });
}

describe('/related/[id] — vector "show all" mirrored in the graph, and legend/lane switches (Task 3)', () => {
  it('collapses the vector lane to 8 rows with a "show all" row, and the graph draws only those 8 vector nodes', async () => {
    relatedMemoriesSpy.mockReset().mockResolvedValue(vectorHitsResponse(20));
    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();

    await expect.poll(() => screen.container.querySelectorAll('[data-testid^="lane-row-vector-"]').length).toBe(8);
    await expect.element(screen.getByText('showing 8 of 20 · k=64 · show all 20 ▸')).toBeInTheDocument();
    expect(screen.container.querySelectorAll('[id^="gn-v"]').length).toBe(8);
  });

  it('expanding "show all" reveals every vector row and node with no second RelatedMemories call', async () => {
    relatedMemoriesSpy.mockReset().mockResolvedValue(vectorHitsResponse(20));
    const screen = await renderRelated();
    await expect.element(screen.getByText('showing 8 of 20 · k=64 · show all 20 ▸')).toBeInTheDocument();

    await screen.getByRole('button', { name: 'showing 8 of 20 · k=64 · show all 20 ▸' }).click();

    await expect.poll(() => screen.container.querySelectorAll('[data-testid^="lane-row-vector-"]').length).toBe(20);
    expect(screen.container.querySelectorAll('[id^="gn-v"]').length).toBe(20);
    expect(relatedMemoriesSpy).toHaveBeenCalledTimes(1);
  });

  it('inserts the ceiling-cut clause in the collapsed row when the response is truncated', async () => {
    relatedMemoriesSpy.mockReset().mockResolvedValue(vectorHitsResponse(20, true));
    const screen = await renderRelated();
    await expect.element(screen.getByText('showing 8 of 20 · k=64 · ceiling cut the rest · show all 20 ▸')).toBeInTheDocument();
  });

  it('unchecking the tag legend checkbox hides the tag lane body, removes tag-only nodes from the graph, and flips the lane button to "show"', async () => {
    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();
    await expect.poll(() => screen.container.querySelector('#gn-tag-uuid')).not.toBeNull();

    await screen.getByRole('checkbox', { name: 'Show tag edges' }).click();

    await expect.poll(() => screen.container.querySelector('#gn-tag-uuid')).toBeNull();
    expect(screen.container.querySelectorAll('[data-testid^="lane-row-tag-"]').length).toBe(0);
    await expect.element(screen.getByTestId('lane-toggle-tag')).toHaveTextContent('show');
  });

  it("pressing the tag lane's hide button unchecks the legend checkbox", async () => {
    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();

    await screen.getByTestId('lane-toggle-tag').click();

    await expect.element(screen.getByRole('checkbox', { name: 'Show tag edges' })).not.toBeChecked();
  });

  it('re-centring resets the vector lane back to collapsed', async () => {
    relatedMemoriesSpy.mockReset().mockResolvedValue(vectorHitsResponse(20));
    const screen = await renderRelated();
    await expect.element(screen.getByText('showing 8 of 20 · k=64 · show all 20 ▸')).toBeInTheDocument();

    await screen.getByRole('button', { name: 'showing 8 of 20 · k=64 · show all 20 ▸' }).click();
    await expect.poll(() => screen.container.querySelectorAll('[data-testid^="lane-row-vector-"]').length).toBe(20);

    await screen.getByTestId('lane-row-vector-v0-uuid').click();
    relatedMemoriesSpy.mockClear().mockResolvedValue(vectorHitsResponse(20));
    await screen.getByRole('button', { name: 'Re-centre ↵' }).click();

    await expect.poll(() => screen.container.querySelectorAll('[data-testid^="lane-row-vector-"]').length).toBe(8);
  });
});

// Fixture: an anchor plus 5 candidates -- 2 carrying #qdrant (one of them
// also a supersession predecessor, for a chain-card dim check), 3 carrying
// #other (one of them also a supersession predecessor). Lets the tag filter
// on #qdrant dim exactly the 3 #other candidates across lane rows, a chain
// card and graph nodes, never the anchor and never a #qdrant carrier.
function tagFilterFixtureResponse() {
  const anchor = makeMemory({ id: 'anchor-uuid', shortId: 'anchor00001', category: 'decision', summary: 'Anchor summary', tags: ['engram'] });

  function tagEdge(tag: string, weight: number) {
    return create(RelatedEdgeSchema, {
      type: EdgeType.TAG,
      evidence: { case: 'tag', value: create(TagEvidenceSchema, { sharedTags: [create(WeightedTagSchema, { tag, weight })], tagWeight: weight }) }
    });
  }
  function supersessionEdge(depth: number) {
    return create(RelatedEdgeSchema, {
      type: EdgeType.SUPERSESSION,
      evidence: { case: 'supersession', value: create(SupersessionEvidenceSchema, { direction: SupersessionDirection.PREDECESSOR, depth }) }
    });
  }

  const qa = makeMemory({ id: 'qa-uuid', shortId: 'qacand00001', category: 'gotcha', summary: 'Qdrant candidate A', tags: ['qdrant'] });
  const qb = makeMemory({ id: 'qb-uuid', shortId: 'qbcand00001', category: 'preference', summary: 'Qdrant candidate B', tags: ['qdrant'] });
  const oa = makeMemory({ id: 'oa-uuid', shortId: 'oacand00001', category: 'discovery', summary: 'Other candidate A', tags: ['other'] });
  const ob = makeMemory({ id: 'ob-uuid', shortId: 'obcand00001', category: 'discovery', summary: 'Other candidate B', tags: ['other'] });
  const oc = makeMemory({ id: 'oc-uuid', shortId: 'occand00001', category: 'discovery', summary: 'Other candidate C', tags: ['other'] });

  return create(RelatedMemoriesResponseSchema, {
    anchor,
    truncated: false,
    related: [
      create(RelatedMemorySchema, { memory: qa, edges: [tagEdge('qdrant', 1.1), supersessionEdge(1)] }),
      create(RelatedMemorySchema, { memory: qb, edges: [tagEdge('qdrant', 1.1)] }),
      create(RelatedMemorySchema, { memory: oa, edges: [tagEdge('other', 0.85)] }),
      create(RelatedMemorySchema, { memory: ob, edges: [tagEdge('other', 0.85)] }),
      create(RelatedMemorySchema, { memory: oc, edges: [tagEdge('other', 0.85), supersessionEdge(2)] })
    ]
  });
}

function tagFilterTagsResponse() {
  return create(ListTagsResponseSchema, {
    tags: [
      { tag: 'engram', count: 1n },
      { tag: 'qdrant', count: 2n },
      { tag: 'other', count: 3n },
      { tag: 'unrelated', count: 50n }
    ],
    more: false
  });
}

describe('/related/[id] — rail Tags tab and the in-view tag filter (Task 1, D-13/D-14/D-15/D-11)', () => {
  beforeEach(() => {
    relatedMemoriesSpy.mockReset().mockResolvedValue(tagFilterFixtureResponse());
    listTagsSpy.mockReset().mockResolvedValue(tagFilterTagsResponse());
  });

  it('shows Graph and Tags rail tabs, defaulting to Graph', async () => {
    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();
    await expect.element(screen.getByRole('tab', { name: 'Graph' })).toHaveAttribute('aria-selected', 'true');
    await expect.element(screen.getByRole('tab', { name: 'Tags' })).toHaveAttribute('aria-selected', 'false');
  });

  it('the Tags tab renders bars from listTags({ scope: "", limit: 1000n }) with the anchor tag marked ●', async () => {
    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();

    await screen.getByRole('tab', { name: 'Tags' }).click();

    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);
    expect(listTagsSpy).toHaveBeenCalledWith({ scope: '', limit: 1000n }, expect.anything());

    const rows = screen.container.querySelectorAll('[role="option"][aria-label]');
    const engramRow = Array.from(rows).find((r) => r.textContent?.includes('engram'));
    expect(engramRow?.classList.contains('marked')).toBe(true);
    expect(engramRow?.textContent).toContain('●');
  });

  it('clicking a tag shows the chip and dims lane rows, a chain card and graph nodes lacking it -- never the anchor, never a carrier', async () => {
    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();

    await screen.getByRole('tab', { name: 'Tags' }).click();
    const qdrantRow = screen.container.querySelector('[role="option"][aria-label^="#qdrant"]') as HTMLElement;
    qdrantRow.click();

    await expect.element(screen.getByTestId('tag-filter-chip')).toHaveTextContent('#qdrant 2 of 5 carry it');

    // Lane rows: #other candidates dim, #qdrant candidates do not.
    await expect.poll(() => screen.container.querySelector('[data-testid="lane-row-tag-oa-uuid"]')?.classList.contains('fdim')).toBe(true);
    expect(screen.container.querySelector('[data-testid="lane-row-tag-ob-uuid"]')?.classList.contains('fdim')).toBe(true);
    expect(screen.container.querySelector('[data-testid="lane-row-tag-qa-uuid"]')?.classList.contains('fdim')).toBe(false);
    expect(screen.container.querySelector('[data-testid="lane-row-tag-qb-uuid"]')?.classList.contains('fdim')).toBe(false);

    // Chain cards: oc-uuid (#other) dims, qa-uuid (#qdrant) does not.
    expect(screen.container.querySelector('[data-testid="chain-card-oc-uuid"]')?.classList.contains('fdim')).toBe(true);
    expect(screen.container.querySelector('[data-testid="chain-card-qa-uuid"]')?.classList.contains('fdim')).toBe(false);
    expect(screen.container.querySelector('[data-testid="chain-card-anchor-uuid"]')?.classList.contains('fdim')).toBe(false);

    // Graph nodes: switch back to Graph to inspect them.
    await screen.getByRole('tab', { name: 'Graph' }).click();
    expect(screen.container.querySelector('#gn-oa-uuid')?.classList.contains('fdim')).toBe(true);
    expect(screen.container.querySelector('#gn-qa-uuid')?.classList.contains('fdim')).toBe(false);
    expect(screen.container.querySelector('#gn-anchor-uuid')?.classList.contains('fdim')).toBe(false);
  });

  it("sets the graph's live summary to end with the filter clause", async () => {
    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();

    await screen.getByRole('tab', { name: 'Tags' }).click();
    (screen.container.querySelector('[role="option"][aria-label^="#qdrant"]') as HTMLElement).click();
    await screen.getByRole('tab', { name: 'Graph' }).click();

    // p.sr-only[aria-live] is the neighbourhood-summary line; the zoom
    // readout is a DIFFERENT aria-live element (div.pct), so the selector
    // must stay tag-specific rather than matching the first [aria-live].
    const live = screen.container.querySelector('p.sr-only[aria-live="polite"]');
    await expect.poll(() => live?.textContent ?? '').toContain('· #qdrant 2 of 5 carry it');
  });

  it('clicking × clears the filter and removes every .fdim', async () => {
    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();

    await screen.getByRole('tab', { name: 'Tags' }).click();
    (screen.container.querySelector('[role="option"][aria-label^="#qdrant"]') as HTMLElement).click();
    await expect.element(screen.getByTestId('tag-filter-chip')).toBeInTheDocument();

    await screen.getByRole('button', { name: 'Clear tag filter' }).click();

    await expect.poll(() => screen.container.querySelector('[data-testid="tag-filter-chip"]')).toBeNull();
    expect(screen.container.querySelectorAll('.fdim').length).toBe(0);
  });

  it('clicking the same tag twice clears the filter', async () => {
    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();

    await screen.getByRole('tab', { name: 'Tags' }).click();
    const qdrantRow = () => screen.container.querySelector('[role="option"][aria-label^="#qdrant"]') as HTMLElement;
    qdrantRow().click();
    await expect.element(screen.getByTestId('tag-filter-chip')).toBeInTheDocument();

    qdrantRow().click();
    await expect.poll(() => screen.container.querySelector('[data-testid="tag-filter-chip"]')).toBeNull();
  });

  it('the tag filter never changes membership -- role=option count is unaffected', async () => {
    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();
    const before = screen.container.querySelectorAll('svg.graph [role="option"]').length;

    await screen.getByRole('tab', { name: 'Tags' }).click();
    (screen.container.querySelector('[role="option"][aria-label^="#qdrant"]') as HTMLElement).click();
    await screen.getByRole('tab', { name: 'Graph' }).click();

    expect(screen.container.querySelectorAll('svg.graph [role="option"]').length).toBe(before);
  });

  it('a tag present in tag-edge evidence shows a rarity line matching the server weight; a tag absent from evidence shows count only', async () => {
    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();

    await screen.getByRole('tab', { name: 'Tags' }).click();
    const qdrantRow = screen.container.querySelector('[role="option"][aria-label^="#qdrant"]') as HTMLElement;
    const unrelatedRow = screen.container.querySelector('[role="option"][aria-label^="#unrelated"]') as HTMLElement;

    expect(qdrantRow.getAttribute('title')).toBe('count 2\nrarity ln(n/df) 1.10');
    expect(unrelatedRow.getAttribute('title')).toBe('count 50');
  });

  it('selecting a lane row while the Tags tab is showing switches the rail to Graph and shows evidence', async () => {
    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();

    await screen.getByRole('tab', { name: 'Tags' }).click();
    await expect.element(screen.getByRole('tab', { name: 'Tags' })).toHaveAttribute('aria-selected', 'true');

    await screen.getByTestId('lane-row-tag-oa-uuid').click();

    await expect.element(screen.getByRole('tab', { name: 'Graph' })).toHaveAttribute('aria-selected', 'true');
    await expect.element(screen.getByRole('region', { name: 'Why oacand00001 is related' })).toBeInTheDocument();
  });
});
