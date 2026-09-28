<script lang="ts">
  // The related-neighbourhood graph (GRAPH-01, GRAPH-03, D-06, D-09, D-20):
  // a Svelte-owned inline SVG. Every <g>/<circle>/<path>/<text> comes from
  // an {#each} -- d3-force supplies only settled positions (graph.ts:
  // settleLayout) and a persistent simulation this component reheats on
  // drag; d3-selection/d3-zoom/d3-drag attach ONLY via `select(el).call(...)`
  // -- they never create or bind a DOM element (no d3-driven DOM creation).
  import {
    ANCHOR_R,
    NODE_R,
    NODE_R_DENSE,
    LABEL_ALL_MAX,
    EDGE_STYLE,
    settleLayout,
    neighbourhoodSummary,
    type GraphNode,
    type GraphEdge,
    type LaneType,
    type SimNode
  } from '$lib/related/graph';
  import {
    SCALE_MIN,
    SCALE_MAX,
    TRANSLATE_EXTENT,
    ZOOM_STEP,
    WHEEL_HINT,
    WHEEL_HINT_MS,
    wheelZoomFilter,
    fitTransform,
    zoomPercent,
    isOutsideView,
    type ZoomTransform
  } from '$lib/related/zoom';
  import { zoom as d3zoom, zoomIdentity } from 'd3-zoom';
  import { drag as d3drag } from 'd3-drag';
  import { select } from 'd3-selection';
  import { SvelteMap } from 'svelte/reactivity';
  import { untrack } from 'svelte';

  // The graph's own viewBox size -- shared by the fit/auto-pan math and the
  // corner controls' geometry.
  const VIEW = { w: 440, h: 380 };

  let {
    anchorId,
    nodes,
    edges,
    selectedId = null,
    dimmedIds,
    onselect,
    onrecenter,
    summary,
    onleave,
    reducedMotion
  }: {
    anchorId: string;
    nodes: GraphNode[];
    edges: GraphEdge[];
    selectedId?: string | null;
    dimmedIds?: ReadonlySet<string>;
    onselect: (id: string | null) => void;
    onrecenter: (id: string) => void;
    summary?: string;
    onleave?: () => void;
    reducedMotion?: boolean;
  } = $props();

  // D-09/D-20: reduced-motion default from the media query when the host
  // doesn't pass one explicitly.
  const reducedMotionValue = $derived(
    reducedMotion ?? (typeof matchMedia !== 'undefined' ? matchMedia('(prefers-reduced-motion: reduce)').matches : false)
  );

  // D-11: the letter glyph shown per edge type in the floating focus card,
  // matching the lane legend's own glyphs (related-and-tags.md, "Edge
  // encoding").
  const TYPE_LETTER: Record<LaneType, string> = { supersession: 'S', citation: 'C', tag: 'T', vector: 'V' };

  // Recreated only when the node/edge arrays themselves change identity
  // (membership change) -- a selection/dim-only re-render reads the same
  // nodes/edges reference and does not touch the simulation. The settle
  // runs synchronously so the first render already has final positions
  // (D-09); there is no tick-driven animation for the INITIAL layout. The
  // simulation itself is kept alive (not re-created) so a drag's brief
  // reheat (alphaTarget) has something to restart -- its 'tick' listener
  // writes every position into `positionsState` on each subsequent tick.
  let simulation: ReturnType<typeof settleLayout>['simulation'] | undefined;
  let simNodesById = new Map<string, SimNode>();
  const positionsState = new SvelteMap<string, { x: number; y: number }>();

  // D-20 refit trigger: identity of the drawn membership (node ids + edge
  // keys) -- a selection/focus/drag change never touches this, so it never
  // refires the settle or the zoom-fit effect below.
  const membershipKey = $derived(nodes.map((n) => n.id).join(',') + '|' + edges.map((e) => e.key).join(','));

  $effect(() => {
    // Read only membershipKey (not `nodes`/`edges` directly, which would
    // also fire this on a same-membership prop replacement) -- settleLayout
    // itself reads the live `nodes`/`edges` closures, which is fine since
    // they always agree with the key at the moment this effect runs.
    void membershipKey;
    const { simulation: sim, simNodes } = settleLayout(nodes, edges);
    simulation = sim;
    simNodesById = new Map(simNodes.map((sn) => [sn.id, sn]));
    // WRITE-ONLY on this reactive map inside this effect -- clear() then
    // set() never READS positionsState (no .keys()/.get() in this body),
    // which is what a mixed read+write of the same reactive collection in
    // one effect run would otherwise trip (Svelte's effect_update_depth_
    // exceeded self-reference guard).
    positionsState.clear();
    for (const sn of simNodes) {
      positionsState.set(sn.id, { x: sn.x ?? 0, y: sn.y ?? 0 });
    }
    sim.on('tick', () => {
      for (const sn of simNodes) {
        positionsState.set(sn.id, { x: sn.x ?? 0, y: sn.y ?? 0 });
      }
    });
  });

  const anchor = $derived(nodes.find((n) => n.isAnchor));
  // Past LABEL_ALL_MAX drawn nodes, radius shrinks and only the anchor and
  // the selected node keep a label (D-20).
  const dense = $derived(nodes.length > LABEL_ALL_MAX);

  // D-20 pan/zoom: `zoomT` is the live transform d3-zoom writes into on
  // every 'zoom' event; the nodes/edges <g> reads it directly for its own
  // `transform` attribute. d3-zoom/d3-selection are used ONLY as
  // `select(svgEl).call(zoomBehavior)` -- no DOM creation, matching D-20's
  // plumbing rule.
  let zoomT = $state<ZoomTransform>({ k: 1, x: 0, y: 0 });
  let zoomBehavior: ReturnType<typeof d3zoom<SVGSVGElement, unknown>> | undefined;
  let boundSvgEl: SVGSVGElement | undefined;

  function computeBounds(): { x0: number; y0: number; x1: number; y1: number } {
    const xs = nodes.map((n) => positionsState.get(n.id)?.x ?? 0);
    const ys = nodes.map((n) => positionsState.get(n.id)?.y ?? 0);
    if (xs.length === 0) return { x0: -1, y0: -1, x1: 1, y1: 1 };
    return { x0: Math.min(...xs), y0: Math.min(...ys), x1: Math.max(...xs), y1: Math.max(...ys) };
  }

  function applyTransform(next: ZoomTransform) {
    if (!svgEl || !zoomBehavior) return;
    select(svgEl).call(zoomBehavior.transform, zoomIdentity.translate(next.x, next.y).scale(next.k));
  }

  // D-20 refit: whenever the drawn membership changes, refit the view to
  // the settled positions' bounds. Selection, focus and drag never trigger
  // this -- the effect's only TRACKED dependencies are `membershipKey` and
  // `svgEl`; `positionsState` is read inside `untrack` on purpose, so a
  // later position write (a drag tick, or this same effect's own
  // `applyTransform` dispatching a 'zoom' event) can never re-trigger it.
  $effect(() => {
    const key = membershipKey;
    if (!svgEl) return;
    if (boundSvgEl !== svgEl) {
      boundSvgEl = svgEl;
      const sel = select(svgEl);
      const z = d3zoom<SVGSVGElement, unknown>()
        .scaleExtent([SCALE_MIN, SCALE_MAX])
        .translateExtent(TRANSLATE_EXTENT as unknown as [[number, number], [number, number]])
        .filter((event: Event) => wheelZoomFilter(event as WheelEvent | MouseEvent))
        .on('zoom', (event) => {
          zoomT = { k: event.transform.k, x: event.transform.x, y: event.transform.y };
        });
      sel.call(z).on('dblclick.zoom', null);
      zoomBehavior = z;
    }
    void key;
    untrack(() => {
      const f = fitTransform(computeBounds(), VIEW);
      applyTransform(f);
    });
  });

  // D-20 wheel gate: d3-zoom's own `.filter()` already rejects a plain
  // wheel (Pattern 2, RESEARCH.md) -- this SEPARATE listener never calls
  // preventDefault, so native page scroll always proceeds; it only flashes
  // the "hold ⌘ to zoom" hint when neither modifier is held.
  let showWheelHint = $state(false);
  let wheelHintTimer: ReturnType<typeof setTimeout> | undefined;

  function onGraphWheel(e: WheelEvent) {
    if (e.ctrlKey || e.metaKey) return;
    showWheelHint = true;
    clearTimeout(wheelHintTimer);
    wheelHintTimer = setTimeout(() => {
      showWheelHint = false;
    }, WHEEL_HINT_MS);
  }

  function zoomIn() {
    if (!svgEl || !zoomBehavior) return;
    select(svgEl).call(zoomBehavior.scaleBy, ZOOM_STEP);
  }
  function zoomOut() {
    if (!svgEl || !zoomBehavior) return;
    select(svgEl).call(zoomBehavior.scaleBy, 1 / ZOOM_STEP);
  }
  function zoomFit() {
    applyTransform(fitTransform(computeBounds(), VIEW));
  }

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
    const idx = nodes.findIndex((n) => n.id === activeId);
    // A stale activeId (the focused node vanished from a props change)
    // resumes navigation from the anchor's baseline (index 0), so the next
    // arrow key moves relative to the anchor rather than snapping back onto
    // the anchor itself.
    return idx >= 0 ? idx : 0;
  }

  function setActiveIndex(idx: number) {
    if (nodes.length === 0) return;
    const clamped = Math.max(0, Math.min(nodes.length - 1, idx));
    activeId = nodes[clamped]?.id ?? null;
  }

  function onGraphFocus() {
    if (activeId === null) activeId = anchor?.id ?? nodes[0]?.id ?? null;
  }

  // D-20 auto-pan: when arrow focus lands on a node outside the current
  // viewport, translateTo it. Depends only on activeId/positionsState (read
  // via untrack for the CURRENT transform) so a zoom-driven change to
  // `zoomT` alone never re-triggers this effect -- translateTo would
  // otherwise re-fire its own 'zoom' event and loop.
  $effect(() => {
    const id = activeId;
    if (id === null) return;
    const pos = positionsState.get(id);
    if (!pos) return;
    untrack(() => {
      if (!svgEl || !zoomBehavior) return;
      if (isOutsideView(pos, zoomT, VIEW)) {
        select(svgEl).call(zoomBehavior.translateTo, pos.x, pos.y);
      }
    });
  });

  // D-11: the visually hidden neighbourhood list and the aria-live summary
  // both read the SAME lane-order `nodes` prop the graph renders -- never a
  // second, independently derived list.
  const liveSummary = $derived(summary ?? neighbourhoodSummary(nodes));

  // D-04 (graph half): Escape first clears the selection, then leaves the
  // graph region -- both tiers stop propagation so a route-level Escape
  // listener never also fires for the same keypress.
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
      case 'Escape':
        e.preventDefault();
        e.stopPropagation();
        if (selectedId !== null) {
          onselect(null);
        } else {
          onleave?.();
        }
        break;
      default:
        break;
    }
  }

  // D-20 focus card: `hoverId` is the pointer-hovered node (takes priority
  // over keyboard focus, matching ResultRow's own hover-over-keyboard
  // precedent); `lastKnownPos` remembers the last node the card rendered for
  // so a props change that drops the focused node (a refetch) can still
  // place its "unreadable now" message where the node used to be, until the
  // next arrow key resumes navigation from the anchor.
  let hoverId = $state<string | null>(null);
  let lastKnownPos = $state<{ id: string; shortId: string; x: number; y: number } | null>(null);

  $effect(() => {
    if (activeId === null) return;
    const pos = positionsState.get(activeId);
    const node = nodes.find((n) => n.id === activeId);
    if (pos && node) {
      lastKnownPos = { id: activeId, shortId: node.shortId, x: pos.x, y: pos.y };
    }
  });

  const isLost = $derived(activeId !== null && !nodes.some((n) => n.id === activeId));

  const cardNode = $derived.by(() => {
    if (hoverId !== null) {
      const n = nodes.find((x) => x.id === hoverId);
      return n && n.id !== selectedId ? n : null;
    }
    if (isLost) return null;
    if (activeId !== null && activeId !== selectedId) {
      return nodes.find((n) => n.id === activeId) ?? null;
    }
    return null;
  });

  function cardPlacement(pos: { x: number; y: number }): { left: string; top: string; flip: boolean } {
    // Maps a node's position through the CURRENT zoom transform into
    // wrapper-relative percentages -- flips left when the transformed x
    // sits past the viewBox's own right half, so the card stays inside the
    // rail at any zoom/pan state.
    const sx = pos.x * zoomT.k + zoomT.x;
    const sy = pos.y * zoomT.k + zoomT.y;
    const flip = sx > 60;
    const leftPct = ((sx + 220) / 440) * 100;
    const topPct = ((sy + 190) / 380) * 100;
    return { left: `${leftPct}%`, top: `${topPct}%`, flip };
  }

  function onNodeMouseEnter(n: GraphNode) {
    hoverId = n.id;
  }
  function onNodeMouseLeave() {
    hoverId = null;
  }

  // D-09 spring-back drag: a Svelte action so `select(node).call(drag())`
  // attaches to the REAL rendered <g> element per non-anchor node -- d3-drag
  // is the call target only, it creates nothing. Start pins the node under
  // the pointer and (unless reducedMotion) briefly reheats the simulation;
  // release always unpins (no pinning persists); under reducedMotion there
  // is no reheat and the node snaps back to its pre-drag position instantly.
  function dragAction(node: SVGGElement, n: GraphNode) {
    if (n.isAnchor) return {};
    let restorePos: { x: number; y: number } | null = null;

    const behavior = d3drag<SVGGElement, unknown>()
      .on('start', () => {
        const simNode = simNodesById.get(n.id);
        if (!simNode) return;
        const cur = positionsState.get(n.id);
        restorePos = cur ? { x: cur.x, y: cur.y } : null;
        if (!reducedMotionValue) {
          simulation?.alphaTarget(0.25).restart();
        }
        simNode.fx = simNode.x;
        simNode.fy = simNode.y;
      })
      .on('drag', (event: { x: number; y: number }) => {
        const simNode = simNodesById.get(n.id);
        if (!simNode) return;
        simNode.fx = event.x;
        simNode.fy = event.y;
        if (reducedMotionValue) {
          simNode.x = event.x;
          simNode.y = event.y;
          positionsState.set(n.id, { x: event.x, y: event.y });
        }
      })
      .on('end', () => {
        const simNode = simNodesById.get(n.id);
        if (!simNode) return;
        simulation?.alphaTarget(0);
        simNode.fx = null;
        simNode.fy = null;
        if (reducedMotionValue && restorePos) {
          simNode.x = restorePos.x;
          simNode.y = restorePos.y;
          positionsState.set(n.id, { x: restorePos.x, y: restorePos.y });
        }
        restorePos = null;
      });

    select(node).call(behavior);
    return {
      destroy() {
        select(node).on('.drag', null);
      }
    };
  }

  function edgePath(e: GraphEdge): string {
    const s = positionsState.get(e.source);
    const t = positionsState.get(e.target);
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
    return !dense || n.isAnchor || selectedId === n.id || activeId === n.id;
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

<div class="gwrap">
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
  onwheel={onGraphWheel}
>
  <defs>
    <marker id="arr" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="6" markerHeight="6" orient="auto">
      <path class="arrow" d="M0,0 L8,4 L0,8 z" />
    </marker>
    <marker id="arr-sel" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="6" markerHeight="6" orient="auto">
      <path class="arrow-sel" d="M0,0 L8,4 L0,8 z" />
    </marker>
  </defs>
  <g class="viewport" transform={`translate(${zoomT.x},${zoomT.y}) scale(${zoomT.k})`}>
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
      {@const pos = positionsState.get(n.id)}
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
        use:dragAction={n}
        onclick={() => nodeClick(n)}
        ondblclick={() => nodeDblClick(n)}
        onmouseenter={() => onNodeMouseEnter(n)}
        onmouseleave={() => onNodeMouseLeave()}
      >
        <circle class="halo" r={r + 4} />
        <circle class="focus" r={r + 6} />
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
  </g>
</svg>

<!-- D-20 corner zoom controls -->
<div class="zctl">
  <button type="button" aria-label="Zoom in" disabled={zoomT.k >= SCALE_MAX} onclick={zoomIn}>+</button>
  <button type="button" aria-label="Zoom out" disabled={zoomT.k <= SCALE_MIN} onclick={zoomOut}>&minus;</button>
  <button type="button" aria-label="Fit graph to view" onclick={zoomFit}>&#10530;</button>
  <div class="pct" aria-live="polite"><span class="sr-only">Zoom</span> {zoomPercent(zoomT.k)}%</div>
</div>

<div class="wheel-hint" class:show={showWheelHint}>{WHEEL_HINT}</div>

<!-- D-11: visually hidden neighbourhood list, one item per node in the same
     lane order the graph draws -- the screen-reader equivalent of the
     visual graph. -->
<ul class="sr-only">
  {#each nodes as n (n.id)}
    <li aria-current={activeId === n.id ? 'true' : undefined}>{n.name}</li>
  {/each}
</ul>
<p class="sr-only" aria-live="polite">{liveSummary}</p>

{#if isLost && lastKnownPos}
  {@const place = cardPlacement(lastKnownPos)}
  <div class="flabel show" class:flip={place.flip} style="left: {place.left}; top: {place.top};">
    No memory with id {lastKnownPos.shortId} that you can read
  </div>
{:else if cardNode}
  {@const pos = positionsState.get(cardNode.id)}
  {#if pos}
    {@const place = cardPlacement(pos)}
    <div class="flabel show" class:flip={place.flip} style="left: {place.left}; top: {place.top};">
      <div class="top">
        <span class="mono">{cardNode.shortId}</span>
        {#each cardNode.types as ty (ty)}<span class="g g-{ty}">{TYPE_LETTER[ty]}</span>{/each}
      </div>
      <div class="sum">{cardNode.summary}</div>
      {#if cardNode.states.length > 0}
        <div class="states">
          {#each cardNode.states as s (s)}<span class="state-word">{s}</span>{/each}
        </div>
      {/if}
    </div>
  {/if}
{/if}
</div>

<style>
  .gwrap {
    position: relative;
  }
  .graph {
    display: block;
    width: 100%;
    height: auto;
  }
  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
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

  /* D-20 keyboard focus ring: dashed violet ring at r + 6, drawn on every
     node but only visible on the currently-focused one (.kfocus). */
  .focus {
    fill: none;
    stroke: none;
  }
  .node.kfocus .focus {
    stroke: var(--primary);
    stroke-width: 1.5;
    stroke-dasharray: 3 2.5;
  }

  /* D-20 floating focus card: shown on keyboard focus or hover, hidden for
     the selected node (its evidence already renders under the graph). */
  .flabel {
    position: absolute;
    pointer-events: none;
    max-width: calc(228 * var(--u));
    background: var(--surface-2);
    border: 1px solid var(--primary);
    border-radius: calc(6 * var(--u));
    padding: calc(4 * var(--u)) calc(6 * var(--u));
    font-size: var(--text-2xs, 11px);
    transform: translate(calc(8 * var(--u)), calc(-50% - 8px));
  }
  .flabel.flip {
    transform: translate(calc(-100% - 8 * var(--u)), calc(-50% - 8px));
  }
  .flabel .top {
    display: flex;
    align-items: center;
    gap: calc(4 * var(--u));
  }
  .flabel .mono {
    font-family: var(--font-mono, monospace);
  }
  .flabel .sum {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .flabel .g {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: calc(16 * var(--u));
    height: calc(16 * var(--u));
    font: 600 calc(9 * var(--u)) / 1 var(--font-mono, monospace);
    color: var(--muted-foreground);
    border: 1px solid var(--muted-foreground);
    border-radius: calc(3 * var(--u));
  }
  .flabel .g-supersession {
    border-width: 2px;
  }
  .flabel .g-citation {
    border-style: dashed;
  }
  .flabel .g-tag {
    border-style: dotted;
    border-width: 2px;
  }
  .flabel .g-vector {
    opacity: 0.7;
  }
  .flabel .state-word {
    font-size: 10px;
    color: var(--muted-foreground);
    margin-right: calc(4 * var(--u));
  }

  /* D-20 corner zoom controls, top-right of the canvas. */
  .zctl {
    position: absolute;
    top: calc(8 * var(--u));
    right: calc(8 * var(--u));
    display: flex;
    flex-direction: column;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: calc(6 * var(--u));
    box-shadow: var(--shadow-sm, 0 1px 2px rgba(0, 0, 0, 0.1));
  }
  .zctl button {
    width: calc(24 * var(--u));
    height: calc(24 * var(--u));
    font: 600 var(--text-xs, 11px) var(--font-mono, monospace);
    color: var(--muted-foreground);
    background: transparent;
    border: none;
    cursor: pointer;
  }
  .zctl button:disabled {
    opacity: 0.4;
    cursor: default;
  }
  .zctl .pct {
    font: calc(11 * var(--u)) var(--font-mono, monospace);
    color: var(--muted-foreground);
    border-top: 1px solid var(--border-subtle);
    text-align: center;
  }

  /* D-20 wheel hint: flashed for WHEEL_HINT_MS when a plain wheel is used
     over the graph -- never intercepts pointer events, never blocks scroll. */
  .wheel-hint {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    pointer-events: none;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: calc(6 * var(--u));
    padding: calc(4 * var(--u)) calc(10 * var(--u));
    font-size: var(--text-xs, 12px);
    opacity: 0;
    transition: opacity 0.15s ease;
  }
  .wheel-hint.show {
    opacity: 1;
  }
</style>
