// app.css is not pulled in by an isolated component mount -- without it the
// category/token CSS variables this graph reads (var(--cat-*),
// var(--muted-foreground), var(--primary), var(--card)) are undefined in
// this test's cascade (same rationale as related.browser.test.ts).
import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import { describe, it, expect, vi } from 'vitest';
import { tick } from 'svelte';
import RelatedGraph from './RelatedGraph.svelte';
import { EDGE_STYLE, LABEL_ALL_MAX, neighbourhoodSummary, type GraphNode, type GraphEdge } from '$lib/related/graph';

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

// Mirrors ResultsList.browser.test.ts's own fireKey helper — dispatches a
// real keydown on the already-focused element so modifier flags (ctrlKey,
// metaKey) can be asserted precisely.
function fireKey(el: Element, key: string, opts: Partial<KeyboardEventInit> = {}) {
  el.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...opts }));
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

describe('RelatedGraph — keyboard traversal (GRAPH-02, D-10)', () => {
  function laneNodes(): GraphNode[] {
    return anchorPlus([
      mkNode('sup', { types: ['supersession'] }),
      mkNode('cit', { types: ['citation'] }),
      mkNode('tag', { types: ['tag'] }),
      mkNode('vec', { types: ['vector'] })
    ]);
  }

  it('focusing the svg sets aria-activedescendant to the anchor', async () => {
    const screen = await renderGraph(laneNodes(), []);
    const svg = screen.container.querySelector('svg')!;
    (svg as HTMLElement).focus();
    await tick();
    expect(svg.getAttribute('aria-activedescendant')).toBe('gn-anchor-id');
  });

  it('ArrowDown/ArrowRight walk forward in lane order, ArrowUp/ArrowLeft walk back, both clamp at the ends, focus stays on the svg', async () => {
    const screen = await renderGraph(laneNodes(), []);
    const svg = screen.container.querySelector('svg')!;
    (svg as HTMLElement).focus();
    await tick();
    expect(svg.getAttribute('aria-activedescendant')).toBe('gn-anchor-id');

    fireKey(svg, 'ArrowDown');
    await tick();
    expect(svg.getAttribute('aria-activedescendant')).toBe('gn-sup');
    fireKey(svg, 'ArrowDown');
    await tick();
    expect(svg.getAttribute('aria-activedescendant')).toBe('gn-cit');
    fireKey(svg, 'ArrowRight');
    await tick();
    expect(svg.getAttribute('aria-activedescendant')).toBe('gn-tag');
    fireKey(svg, 'ArrowRight');
    await tick();
    expect(svg.getAttribute('aria-activedescendant')).toBe('gn-vec');
    // clamp at the end
    fireKey(svg, 'ArrowDown');
    await tick();
    expect(svg.getAttribute('aria-activedescendant')).toBe('gn-vec');

    fireKey(svg, 'ArrowUp');
    await tick();
    expect(svg.getAttribute('aria-activedescendant')).toBe('gn-tag');
    fireKey(svg, 'ArrowLeft');
    await tick();
    expect(svg.getAttribute('aria-activedescendant')).toBe('gn-cit');
    fireKey(svg, 'ArrowLeft');
    await tick();
    expect(svg.getAttribute('aria-activedescendant')).toBe('gn-sup');
    fireKey(svg, 'ArrowUp');
    await tick();
    expect(svg.getAttribute('aria-activedescendant')).toBe('gn-anchor-id');
    // clamp at the start
    fireKey(svg, 'ArrowUp');
    await tick();
    expect(svg.getAttribute('aria-activedescendant')).toBe('gn-anchor-id');

    expect(document.activeElement).toBe(svg);
  });

  it('End/Home jump to the last/first node', async () => {
    const screen = await renderGraph(laneNodes(), []);
    const svg = screen.container.querySelector('svg')!;
    (svg as HTMLElement).focus();
    await tick();

    fireKey(svg, 'End');
    await tick();
    expect(svg.getAttribute('aria-activedescendant')).toBe('gn-vec');
    fireKey(svg, 'Home');
    await tick();
    expect(svg.getAttribute('aria-activedescendant')).toBe('gn-anchor-id');
  });

  it('Space on a candidate calls onselect(its id); Space on the anchor calls onselect(null)', async () => {
    const onselect = vi.fn();
    const screen = await renderGraph(laneNodes(), [], { onselect });
    const svg = screen.container.querySelector('svg')!;
    (svg as HTMLElement).focus();

    fireKey(svg, 'ArrowDown');
    fireKey(svg, ' ');
    expect(onselect).toHaveBeenCalledWith('sup');

    onselect.mockClear();
    fireKey(svg, 'Home');
    fireKey(svg, ' ');
    expect(onselect).toHaveBeenCalledWith(null);
  });

  it('Enter on a candidate calls onrecenter(its id); Enter on the anchor does nothing', async () => {
    const onrecenter = vi.fn();
    const screen = await renderGraph(laneNodes(), [], { onrecenter });
    const svg = screen.container.querySelector('svg')!;
    (svg as HTMLElement).focus();

    fireKey(svg, 'ArrowDown');
    fireKey(svg, 'Enter');
    expect(onrecenter).toHaveBeenCalledWith('sup');

    onrecenter.mockClear();
    fireKey(svg, 'Home');
    fireKey(svg, 'Enter');
    expect(onrecenter).not.toHaveBeenCalled();
  });

  it('ArrowDown with ctrlKey does not move', async () => {
    const screen = await renderGraph(laneNodes(), []);
    const svg = screen.container.querySelector('svg')!;
    (svg as HTMLElement).focus();
    await tick();
    expect(svg.getAttribute('aria-activedescendant')).toBe('gn-anchor-id');

    fireKey(svg, 'ArrowDown', { ctrlKey: true });
    await tick();
    expect(svg.getAttribute('aria-activedescendant')).toBe('gn-anchor-id');
  });
});

describe('RelatedGraph — screen-reader list and live summary (D-11)', () => {
  function laneNodes(): GraphNode[] {
    return anchorPlus([
      mkNode('sup', { types: ['supersession'] }),
      mkNode('cit', { types: ['citation'] }),
      mkNode('tag', { types: ['tag'] }),
      mkNode('vec', { types: ['vector'] })
    ]);
  }

  it('the hidden list has one item per node in lane order with each accessible name, and the active item carries aria-current', async () => {
    const screen = await renderGraph(laneNodes(), []);
    const svg = screen.container.querySelector('svg')!;
    (svg as HTMLElement).focus();
    await tick();
    fireKey(svg, 'ArrowDown');
    await tick();

    const items = screen.container.querySelectorAll('ul.sr-only li');
    expect(items.length).toBe(5);
    expect(items[0]?.textContent).toBe('anchor-id name');
    expect(items[1]?.textContent).toBe('sup name');
    expect(items[1]?.getAttribute('aria-current')).toBe('true');
    expect(items[0]?.getAttribute('aria-current')).not.toBe('true');
  });

  it('the aria-live region reads neighbourhoodSummary(nodes) by default and updates when nodes changes', async () => {
    const nodes = laneNodes();
    const screen = await renderGraph(nodes, []);
    const live = screen.container.querySelector('p.sr-only[aria-live="polite"]');
    expect(live?.textContent).toBe(neighbourhoodSummary(nodes));

    await screen.rerender({ anchorId: 'anchor-id', nodes: anchorPlus([mkNode('vec', { types: ['vector'] })]), edges: [], onselect: vi.fn(), onrecenter: vi.fn() });
    const nextNodes = anchorPlus([mkNode('vec', { types: ['vector'] })]);
    expect(live?.textContent).toBe(neighbourhoodSummary(nextNodes));
  });

  it('the summary prop overrides the default neighbourhoodSummary text', async () => {
    const screen = await renderGraph(laneNodes(), [], { summary: 'custom announcement' });
    const live = screen.container.querySelector('p.sr-only[aria-live="polite"]');
    expect(live?.textContent).toBe('custom announcement');
  });
});

describe('RelatedGraph — Escape tiers (D-04)', () => {
  it('Escape with a selection calls onselect(null), not onleave, and stops propagation', async () => {
    const nodes = anchorPlus([mkNode('n1', { types: ['tag'] })]);
    const onselect = vi.fn();
    const onleave = vi.fn();
    const windowKeydown = vi.fn();
    window.addEventListener('keydown', windowKeydown);
    const screen = await renderGraph(nodes, [], { selectedId: 'n1', onselect, onleave });
    const svg = screen.container.querySelector('svg')!;
    (svg as HTMLElement).focus();

    fireKey(svg, 'Escape');
    expect(onselect).toHaveBeenCalledWith(null);
    expect(onleave).not.toHaveBeenCalled();
    expect(windowKeydown).not.toHaveBeenCalled();
    window.removeEventListener('keydown', windowKeydown);
  });

  it('Escape with no selection calls onleave, and stops propagation', async () => {
    const nodes = anchorPlus([mkNode('n1', { types: ['tag'] })]);
    const onselect = vi.fn();
    const onleave = vi.fn();
    const windowKeydown = vi.fn();
    window.addEventListener('keydown', windowKeydown);
    const screen = await renderGraph(nodes, [], { onselect, onleave });
    const svg = screen.container.querySelector('svg')!;
    (svg as HTMLElement).focus();

    fireKey(svg, 'Escape');
    expect(onleave).toHaveBeenCalledOnce();
    expect(onselect).not.toHaveBeenCalled();
    expect(windowKeydown).not.toHaveBeenCalled();
    window.removeEventListener('keydown', windowKeydown);
  });
});

describe('RelatedGraph — focus ring and floating focus card (D-20)', () => {
  function laneNodes(): GraphNode[] {
    return anchorPlus([
      mkNode('sup', { types: ['supersession'], summary: 'a supersession candidate' }),
      mkNode('cit', { types: ['citation'] })
    ]);
  }

  it('ArrowDown shows the focus card with the active node short_id and summary', async () => {
    const screen = await renderGraph(laneNodes(), []);
    const svg = screen.container.querySelector('svg')!;
    (svg as HTMLElement).focus();
    await tick();
    fireKey(svg, 'ArrowDown');
    await tick();

    const card = screen.container.querySelector('.flabel.show');
    expect(card).not.toBeNull();
    expect(card?.textContent).toContain('sup0000000'); // shortId
    expect(card?.textContent).toContain('a supersession candidate');
  });

  it('hovering another node shows its card', async () => {
    const nodes = laneNodes();
    const screen = await renderGraph(nodes, []);
    const citEl = screen.container.querySelector('#gn-cit')!;
    citEl.dispatchEvent(new MouseEvent('mouseenter', { bubbles: true }));
    await tick();

    const card = screen.container.querySelector('.flabel.show');
    expect(card).not.toBeNull();
    expect(card?.textContent).toContain('cit name');

    citEl.dispatchEvent(new MouseEvent('mouseleave', { bubbles: true }));
    await tick();
    expect(screen.container.querySelector('.flabel.show')).toBeNull();
  });

  it('the card is absent for the selected node', async () => {
    const nodes = laneNodes();
    const screen = await renderGraph(nodes, [], { selectedId: 'sup' });
    const svg = screen.container.querySelector('svg')!;
    (svg as HTMLElement).focus();
    await tick();
    fireKey(svg, 'ArrowDown'); // moves activeId to 'sup', which is selected
    await tick();

    expect(screen.container.querySelector('.flabel.show')).toBeNull();
  });

  it('with 30 nodes, the focused node label renders past LABEL_ALL_MAX', async () => {
    const many: GraphNode[] = [];
    for (let i = 0; i < LABEL_ALL_MAX + 3; i++) many.push(mkNode(`n${i}`, { types: ['vector'] }));
    const nodes = anchorPlus(many);
    expect(nodes.length).toBe(30);
    const screen = await renderGraph(nodes, []);
    const svg = screen.container.querySelector('svg')!;
    (svg as HTMLElement).focus();
    await tick();
    fireKey(svg, 'ArrowDown');
    await tick();
    fireKey(svg, 'ArrowDown');
    await tick();

    const focusedLabel = screen.container.querySelector('#gn-n1 text.lbl');
    expect(focusedLabel).not.toBeNull();
  });

  it('a refetch that drops the focused node shows the not-found message, and the next arrow key resumes from the anchor', async () => {
    const nodes = laneNodes();
    const screen = await renderGraph(nodes, []);
    const svg = screen.container.querySelector('svg')!;
    (svg as HTMLElement).focus();
    await tick();
    fireKey(svg, 'ArrowDown'); // activeId = 'sup'
    await tick();

    const nextNodes = anchorPlus([mkNode('cit', { types: ['citation'] })]); // 'sup' dropped
    await screen.rerender({ anchorId: 'anchor-id', nodes: nextNodes, edges: [], onselect: vi.fn(), onrecenter: vi.fn() });
    await tick();

    const card = screen.container.querySelector('.flabel.show');
    expect(card?.textContent).toContain('No memory with id');
    expect(card?.textContent).toContain('that you can read');

    fireKey(svg, 'ArrowDown'); // resumes from the anchor -> lands on the first candidate
    await tick();
    expect(svg.getAttribute('aria-activedescendant')).toBe('gn-cit');
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
