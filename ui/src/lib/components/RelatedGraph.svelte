<script lang="ts">
  // The related-neighbourhood graph (GRAPH-01, D-06, D-09): a Svelte-owned
  // inline SVG. Every <g>/<circle>/<path>/<text> comes from an {#each} --
  // d3-force supplies only the settled positions (graph.ts:settleLayout);
  // d3-selection/d3-zoom/d3-drag are not used in this task (roving keyboard,
  // pan/zoom and drag land in a later plan per the read_first note).
  import { ANCHOR_R, NODE_R, settleLayout, type GraphNode, type GraphEdge } from '$lib/related/graph';

  let {
    anchorId,
    nodes,
    edges,
    selectedId = null,
    dimmedIds,
    onselect,
    onrecenter
  }: {
    anchorId: string;
    nodes: GraphNode[];
    edges: GraphEdge[];
    selectedId?: string | null;
    dimmedIds?: ReadonlySet<string>;
    onselect: (id: string | null) => void;
    onrecenter: (id: string) => void;
  } = $props();

  // Recomputed only when the node/edge arrays themselves change identity
  // (membership change) -- a selection/dim-only re-render reads the same
  // nodes/edges reference and skips the settle entirely. The settle runs
  // synchronously so the first render already has final positions (D-09);
  // there is no tick-driven layout animation.
  const positions = $derived.by(() => {
    const { simNodes } = settleLayout(nodes, edges);
    const map = new Map<string, { x: number; y: number }>();
    for (const n of simNodes) map.set(n.id, { x: n.x ?? 0, y: n.y ?? 0 });
    return map;
  });

  const anchor = $derived(nodes.find((n) => n.isAnchor));

  function edgePath(e: GraphEdge): string {
    const s = positions.get(e.source);
    const t = positions.get(e.target);
    if (!s || !t) return '';
    if (!e.offset) return `M${s.x},${s.y}L${t.x},${t.y}`;
    const dx = t.x - s.x;
    const dy = t.y - s.y;
    const len = Math.hypot(dx, dy) || 1;
    const mx = (s.x + t.x) / 2 - (dy / len) * e.offset;
    const my = (s.y + t.y) / 2 + (dx / len) * e.offset;
    return `M${s.x},${s.y}Q${mx},${my} ${t.x},${t.y}`;
  }

  function radiusFor(n: GraphNode): number {
    return n.isAnchor ? ANCHOR_R : NODE_R;
  }

  function nodeClick(n: GraphNode) {
    onselect(n.isAnchor ? null : n.id);
  }

  function nodeDblClick(n: GraphNode) {
    if (!n.isAnchor) onrecenter(n.id);
  }
</script>

<svg
  class="graph"
  viewBox="-220 -190 440 380"
  role="listbox"
  tabindex="0"
  aria-label={`Related graph for ${anchor?.shortId ?? anchorId}: ${Math.max(nodes.length - 1, 0)} neighbours`}
>
  <g class="edges">
    {#each edges as e (e.key)}
      <path class="edge t-{e.type}" d={edgePath(e)} fill="none" />
    {/each}
  </g>
  <g class="nodes">
    {#each nodes as n (n.id)}
      {@const pos = positions.get(n.id)}
      <!-- WAI-ARIA APG listbox: options are NOT tab stops and take no
           keyboard handler of their own -- the container (role="listbox")
           owns all keyboard interaction via aria-activedescendant, added in
           a later plan; click is a supplementary pointer affordance. -->
      <!-- svelte-ignore a11y_interactive_supports_focus -->
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <g
        class="node"
        class:anchor={n.isAnchor}
        id="gn-{n.id}"
        role="option"
        aria-selected={selectedId === n.id}
        aria-label={n.name}
        transform={`translate(${pos?.x ?? 0}, ${pos?.y ?? 0})`}
        onclick={() => nodeClick(n)}
        ondblclick={() => nodeDblClick(n)}
      >
        <circle class="body" r={radiusFor(n)} fill="var(--cat-{n.category})" />
        <text class="lbl" text-anchor="middle" y={radiusFor(n) + 10}>{n.shortId}</text>
      </g>
    {/each}
  </g>
</svg>

<style>
  .graph {
    display: block;
    width: 100%;
    height: auto;
  }
  .node {
    cursor: pointer;
  }
  .node.anchor {
    cursor: default;
  }
  .lbl {
    font: var(--text-2xs, calc(11 * var(--u))) / 1 var(--font-mono, monospace);
    fill: var(--muted-foreground);
    paint-order: stroke;
    stroke: var(--card);
    stroke-width: 3px;
  }
</style>
