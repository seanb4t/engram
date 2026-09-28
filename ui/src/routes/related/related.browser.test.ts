import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
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
