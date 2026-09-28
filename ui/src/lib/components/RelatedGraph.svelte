<script lang="ts">
  // The related-neighbourhood graph (GRAPH-01, GRAPH-03, D-06, D-09, D-20
  // label half): a Svelte-owned inline SVG. Every <g>/<circle>/<path>/<text>
  // comes from an {#each} -- d3-force supplies only the settled positions
  // (graph.ts:settleLayout); d3-selection/d3-zoom/d3-drag are not used in
  // this plan (roving keyboard, pan/zoom and drag land in a later plan).
  import {
    ANCHOR_R,
    NODE_R,
    NODE_R_DENSE,
    LABEL_ALL_MAX,
    EDGE_STYLE,
    settleLayout,
    type GraphNode,
    type GraphEdge
  } from '$lib/related/graph';

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
  // Past LABEL_ALL_MAX drawn nodes, radius shrinks and only the anchor and
  // the selected node keep a label (D-20).
  const dense = $derived(nodes.length > LABEL_ALL_MAX);

  // D-10 roving-focus keyboard model: the svg is the ONE tab stop
  // (role=listbox, tabindex=0); activeId walks `nodes` in lane order (the
  // prop's own order -- anchor first, then supersession > citation > tag >
  // vector, strength within lane) exactly as ResultsList.svelte's listbox
  // walks its own array. Focusing the svg seeds activeId to the anchor when
  // unset; an $effect mirrors activeId into aria-activedescendant, mirroring
  // ResultsList's own pattern.
  let svgEl: SVGSVGElement | undefined = $state();
  let activeId = $state<string | null>(null);

  $effect(() => {
    if (!svgEl) return;
    if (activeId !== null) {
      svgEl.setAttribute('aria-activedescendant', `gn-${activeId}`);
    } else {
      svgEl.removeAttribute('aria-activedescendant');
    }
  });

  function activeIndex(): number {
    if (activeId === null) return -1;
    return nodes.findIndex((n) => n.id === activeId);
  }

  function setActiveIndex(idx: number) {
    if (nodes.length === 0) return;
    const clamped = Math.max(0, Math.min(nodes.length - 1, idx));
    activeId = nodes[clamped]?.id ?? null;
  }

  function onGraphFocus() {
    if (activeId === null) activeId = anchor?.id ?? nodes[0]?.id ?? null;
  }

  function onGraphKeydown(e: KeyboardEvent) {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    const current = activeIndex();
    switch (e.key) {
      case 'ArrowDown':
      case 'ArrowRight':
        e.preventDefault();
        setActiveIndex((current < 0 ? -1 : current) + 1);
        break;
      case 'ArrowUp':
      case 'ArrowLeft':
        e.preventDefault();
        setActiveIndex((current < 0 ? 1 : current) - 1);
        break;
      case 'Home':
        e.preventDefault();
        setActiveIndex(0);
        break;
      case 'End':
        e.preventDefault();
        setActiveIndex(nodes.length - 1);
        break;
      case ' ': {
        e.preventDefault();
        const n = current >= 0 ? nodes[current] : undefined;
        if (n) onselect(n.isAnchor ? null : n.id);
        break;
      }
      case 'Enter': {
        e.preventDefault();
        const n = current >= 0 ? nodes[current] : undefined;
        if (n && !n.isAnchor) onrecenter(n.id);
        break;
      }
      default:
        break;
    }
  }

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
    if (n.isAnchor) return ANCHOR_R;
    return dense ? NODE_R_DENSE : NODE_R;
  }

  function showLabel(n: GraphNode): boolean {
    return !dense || n.isAnchor || selectedId === n.id;
  }

  function edgeConnectsSelection(e: GraphEdge): boolean {
    return selectedId !== null && (e.source === selectedId || e.target === selectedId);
  }

  function nodeClick(n: GraphNode) {
    activeId = n.id;
    svgEl?.focus({ preventScroll: true });
    onselect(n.isAnchor ? null : n.id);
  }

  function nodeDblClick(n: GraphNode) {
    if (!n.isAnchor) onrecenter(n.id);
  }
</script>

<svg
  bind:this={svgEl}
  class="graph"
  class:has-sel={selectedId !== null}
  viewBox="-220 -190 440 380"
  role="listbox"
  tabindex="0"
  aria-label={`Related graph for ${anchor?.shortId ?? anchorId}: ${Math.max(nodes.length - 1, 0)} neighbours. Arrows move, Space selects, Enter re-centres, Escape returns.`}
  onfocus={onGraphFocus}
  onkeydown={onGraphKeydown}
>
  <defs>
    <marker id="arr" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="6" markerHeight="6" orient="auto">
      <path class="arrow" d="M0,0 L8,4 L0,8 z" />
    </marker>
    <marker id="arr-sel" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="6" markerHeight="6" orient="auto">
      <path class="arrow-sel" d="M0,0 L8,4 L0,8 z" />
    </marker>
  </defs>
  <g class="edges">
    {#each edges as e (e.key)}
      {@const style = EDGE_STYLE[e.type]}
      {@const onSel = edgeConnectsSelection(e)}
      <path
        class="edge t-{e.type}"
        class:selon={onSel}
        d={edgePath(e)}
        fill="none"
        stroke-width={style.width}
        stroke-dasharray={style.dash || null}
        stroke-linecap={style.cap ?? null}
        marker-end={e.arrow ? (onSel ? 'url(#arr-sel)' : 'url(#arr)') : null}
      />
    {/each}
  </g>
  <g class="nodes">
    {#each nodes as n (n.id)}
      {@const pos = positions.get(n.id)}
      {@const r = radiusFor(n)}
      <!-- WAI-ARIA APG listbox: options are NOT tab stops and take no
           keyboard handler of their own -- the container (role="listbox")
           owns all keyboard interaction via aria-activedescendant, added in
           a later plan; click is a supplementary pointer affordance. -->
      <!-- svelte-ignore a11y_interactive_supports_focus -->
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <g
        class="node"
        class:anchor={n.isAnchor}
        class:sel={selectedId === n.id}
        class:fdim={dimmedIds?.has(n.id)}
        class:hidden-state={n.states.length > 0}
        class:kfocus={activeId === n.id}
        id="gn-{n.id}"
        role="option"
        aria-selected={selectedId === n.id}
        aria-label={n.name}
        transform={`translate(${pos?.x ?? 0}, ${pos?.y ?? 0})`}
        onclick={() => nodeClick(n)}
        ondblclick={() => nodeDblClick(n)}
      >
        <circle class="halo" r={r + 4} />
        {#if n.isAnchor}
          <circle class="ring-b" r={r + 5} />
          <circle class="ring-a" r={r + 2.5} />
        {/if}
        <circle class="body" r={r} fill="var(--cat-{n.category})" />
        {#if showLabel(n)}
          <text class="lbl" text-anchor="middle" y={r + 10}>{n.shortId}</text>
        {/if}
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

  /* edges: grey, type carried by line style only -- selection is the only
     thing allowed to colour an edge (violet). */
  .edge {
    stroke: var(--muted-foreground);
    transition:
      opacity 0.15s ease,
      stroke 0.15s ease;
  }
  .edge.t-vector {
    stroke-opacity: 0.55;
  }
  .edge.selon {
    stroke: var(--primary);
  }
  .arrow {
    fill: var(--muted-foreground);
  }
  .arrow-sel {
    fill: var(--primary);
  }

  /* selection halo + dim */
  .halo {
    fill: none;
    stroke: none;
  }
  .node.sel .halo {
    stroke: var(--primary);
  }
  svg.has-sel .node:not(.sel):not(.anchor) {
    opacity: 0.45;
  }
  .fdim {
    opacity: 0.3;
  }

  /* anchor double ring */
  .ring-a,
  .ring-b {
    fill: none;
    stroke: var(--primary);
  }

  /* a chain member whose record carries state words (archived, superseded,
     expired, scheduled) renders dashed and faded, matching the
     supersession-lane chain-card convention. */
  .node.hidden-state circle {
    opacity: 0.45;
    stroke-dasharray: 2 2;
  }

  .lbl {
    font-size: 9px;
    font-family: var(--font-mono, monospace);
    fill: var(--muted-foreground);
    paint-order: stroke;
    stroke: var(--card);
    stroke-width: 3px;
  }
</style>
