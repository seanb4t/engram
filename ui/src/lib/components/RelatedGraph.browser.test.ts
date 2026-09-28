// app.css is not pulled in by an isolated component mount -- without it the
// category/token CSS variables this graph reads (var(--cat-*),
// var(--muted-foreground), var(--primary), var(--card)) are undefined in
// this test's cascade (same rationale as related.browser.test.ts).
import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import { describe, it, expect, vi } from 'vitest';
import RelatedGraph from './RelatedGraph.svelte';
import { EDGE_STYLE, LABEL_ALL_MAX, type GraphNode, type GraphEdge } from '$lib/related/graph';

function mkNode(id: string, overrides: Partial<GraphNode> = {}): GraphNode {
  return {
    id,
    shortId: id.slice(0, 10).padEnd(10, '0'),
    category: 'convention',
    summary: '',
    isAnchor: false,
    types: [],
    states: [],
    name: `${id} name`,
    ...overrides
  };
}

function mkAnchor(overrides: Partial<GraphNode> = {}): GraphNode {
  return mkNode('anchor-id', { isAnchor: true, category: 'decision', ...overrides });
}

function anchorPlus(rest: GraphNode[]): GraphNode[] {
  return [mkAnchor(), ...rest];
}

function renderGraph(nodes: GraphNode[], edges: GraphEdge[], extra: Record<string, unknown> = {}) {
  return render(RelatedGraph, {
    anchorId: 'anchor-id',
    nodes,
    edges,
    onselect: vi.fn(),
    onrecenter: vi.fn(),
    ...extra
  });
}

describe('RelatedGraph — option count and identity', () => {
  it('renders one role=option per node, with ids gn-{uuid}', async () => {
    const nodes = anchorPlus([mkNode('n1', { types: ['tag'] })]);
    const screen = await renderGraph(nodes, []);
    expect(screen.container.querySelectorAll('[role="option"]').length).toBe(2);
    expect(screen.container.querySelector('#gn-anchor-id')).not.toBeNull();
    expect(screen.container.querySelector('#gn-n1')).not.toBeNull();
  });
});

describe('RelatedGraph — edge encoding (GRAPH-03)', () => {
  it('each edge path carries class t-{type} with the stroke-dasharray/width of EDGE_STYLE, and a supersession path carries the arrow marker', async () => {
    const nodes = anchorPlus([
      mkNode('sup', { types: ['supersession'] }),
      mkNode('cit', { types: ['citation'] }),
      mkNode('tag', { types: ['tag'] }),
      mkNode('vec', { types: ['vector'] })
    ]);
    const edges: GraphEdge[] = [
      { key: 'e-sup', source: 'sup', target: 'anchor-id', type: 'supersession', offset: 0, arrow: true, chain: false },
      { key: 'e-cit', source: 'anchor-id', target: 'cit', type: 'citation', offset: 0, arrow: false, chain: false },
      { key: 'e-tag', source: 'anchor-id', target: 'tag', type: 'tag', offset: 0, arrow: false, chain: false },
      { key: 'e-vec', source: 'anchor-id', target: 'vec', type: 'vector', offset: 0, arrow: false, chain: false }
    ];
    const screen = await renderGraph(nodes, edges);

    const supPath = screen.container.querySelector('path.t-supersession');
    expect(supPath).not.toBeNull();
    expect(supPath?.getAttribute('marker-end')).toBe('url(#arr)');
    expect(supPath?.getAttribute('stroke-width')).toBe(String(EDGE_STYLE.supersession.width));

    const citPath = screen.container.querySelector('path.t-citation');
    expect(citPath?.getAttribute('stroke-dasharray')).toBe(EDGE_STYLE.citation.dash);
    expect(citPath?.getAttribute('stroke-width')).toBe(String(EDGE_STYLE.citation.width));

    const tagPath = screen.container.querySelector('path.t-tag');
    expect(tagPath?.getAttribute('stroke-dasharray')).toBe(EDGE_STYLE.tag.dash);
    expect(tagPath?.getAttribute('stroke-width')).toBe(String(EDGE_STYLE.tag.width));
    expect(tagPath?.getAttribute('stroke-linecap')).toBe('round');

    const vecPath = screen.container.querySelector('path.t-vector');
    expect(vecPath?.getAttribute('stroke-width')).toBe(String(EDGE_STYLE.vector.width));
    expect(vecPath?.getAttribute('marker-end')).toBeNull();
  });
});

describe('RelatedGraph — anchor rings', () => {
  it('the anchor has two ring circles', async () => {
    const screen = await renderGraph([mkAnchor()], []);
    const anchorG = screen.container.querySelector('#gn-anchor-id');
    expect(anchorG?.querySelectorAll('circle.ring-a, circle.ring-b').length).toBe(2);
  });

  it('a non-anchor node has no ring circles', async () => {
    const nodes = anchorPlus([mkNode('n1', { types: ['tag'] })]);
    const screen = await renderGraph(nodes, []);
    const nodeG = screen.container.querySelector('#gn-n1');
    expect(nodeG?.querySelectorAll('circle.ring-a, circle.ring-b').length).toBe(0);
  });
});

describe('RelatedGraph — selection', () => {
  it('a selected id gets .sel, its edges get .selon, and the svg gets .has-sel', async () => {
    const nodes = anchorPlus([mkNode('n1', { types: ['tag'] })]);
    const edges: GraphEdge[] = [{ key: 'e1', source: 'anchor-id', target: 'n1', type: 'tag', offset: 0, arrow: false, chain: false }];
    const screen = await renderGraph(nodes, edges, { selectedId: 'n1' });
    expect(screen.container.querySelector('#gn-n1')?.classList.contains('sel')).toBe(true);
    expect(screen.container.querySelector('path.t-tag')?.classList.contains('selon')).toBe(true);
    expect(screen.container.querySelector('svg')?.classList.contains('has-sel')).toBe(true);
  });
});

describe('RelatedGraph — dimming', () => {
  it('ids in dimmedIds get .fdim', async () => {
    const nodes = anchorPlus([mkNode('n1', { types: ['tag'] })]);
    const screen = await renderGraph(nodes, [], { dimmedIds: new Set(['n1']) });
    expect(screen.container.querySelector('#gn-n1')?.classList.contains('fdim')).toBe(true);
    expect(screen.container.querySelector('#gn-anchor-id')?.classList.contains('fdim')).toBe(false);
  });
});

describe('RelatedGraph — hidden-state nodes', () => {
  it('a node carrying state words gets .hidden-state', async () => {
    const nodes = anchorPlus([mkNode('n1', { types: ['tag'], states: ['superseded'] })]);
    const screen = await renderGraph(nodes, []);
    expect(screen.container.querySelector('#gn-n1')?.classList.contains('hidden-state')).toBe(true);
    expect(screen.container.querySelector('#gn-anchor-id')?.classList.contains('hidden-state')).toBe(false);
  });
});

describe('RelatedGraph — label density (D-20)', () => {
  function manyNodes(count: number): GraphNode[] {
    const nodes: GraphNode[] = [];
    for (let i = 0; i < count; i++) nodes.push(mkNode(`n${i}`, { types: ['vector'] }));
    return anchorPlus(nodes);
  }

  it('26 drawn nodes render 26 labels', async () => {
    const nodes = manyNodes(LABEL_ALL_MAX - 1); // + anchor = 26
    expect(nodes).toHaveLength(LABEL_ALL_MAX);
    const screen = await renderGraph(nodes, []);
    expect(screen.container.querySelectorAll('text.lbl').length).toBe(LABEL_ALL_MAX);
  });

  it('27 drawn nodes label only the anchor and the selected node', async () => {
    const nodes = manyNodes(LABEL_ALL_MAX); // + anchor = 27
    expect(nodes).toHaveLength(LABEL_ALL_MAX + 1);
    const screen = await renderGraph(nodes, [], { selectedId: 'n0' });
    expect(screen.container.querySelectorAll('text.lbl').length).toBe(2);
  });
});

describe('RelatedGraph — click and dblclick', () => {
  it('clicking a candidate calls onselect(id); clicking the anchor calls onselect(null); dblclick on a candidate calls onrecenter(id)', async () => {
    const nodes = anchorPlus([mkNode('n1', { types: ['tag'] })]);
    const onselect = vi.fn();
    const onrecenter = vi.fn();
    const screen = await renderGraph(nodes, [], { onselect, onrecenter });

    screen.container.querySelector('#gn-n1')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(onselect).toHaveBeenCalledWith('n1');

    screen.container.querySelector('#gn-n1')?.dispatchEvent(new MouseEvent('dblclick', { bubbles: true }));
    expect(onrecenter).toHaveBeenCalledWith('n1');

    onselect.mockClear();
    screen.container.querySelector('#gn-anchor-id')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(onselect).toHaveBeenCalledWith(null);
  });
});

describe('RelatedGraph — both themes', () => {
  it('renders correctly in light and dark themes', async () => {
    const nodes = anchorPlus([mkNode('n1', { types: ['tag'] }), mkNode('n2', { types: ['supersession'] })]);
    const edges: GraphEdge[] = [
      { key: 'e1', source: 'anchor-id', target: 'n1', type: 'tag', offset: 0, arrow: false, chain: false },
      { key: 'e2', source: 'n2', target: 'anchor-id', type: 'supersession', offset: 0, arrow: true, chain: false }
    ];
    await renderGraph(nodes, edges, { selectedId: 'n1' });

    document.documentElement.classList.remove('dark');
    await page.screenshot();
    document.documentElement.classList.add('dark');
    await page.screenshot();
    document.documentElement.classList.remove('dark');
  });
});
