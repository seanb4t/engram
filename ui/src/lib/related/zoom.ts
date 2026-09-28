// Pure zoom gate, fit and readout math (GRAPH-01, D-09, D-20). This module
// owns no DOM and no d3-zoom instance -- RelatedGraph.svelte wires these
// functions into a real d3-zoom behavior via .filter()/.on('zoom', ...) and
// the corner-control click handlers. Kept dependency-free (no d3 import)
// so it stays trivially node-testable, mirroring related/graph.ts's split.

export const SCALE_MIN = 0.5;
export const SCALE_MAX = 4;
// Matches the graph's own viewBox extent (-220..220, -190..190) -- panning
// never carries the graph further than its own drawn bounds.
export const TRANSLATE_EXTENT: readonly [readonly [number, number], readonly [number, number]] = [
  [-440, -380],
  [440, 380]
];
export const ZOOM_STEP = 1.25;
export const WHEEL_HINT = 'hold ⌘ to zoom with the wheel';
export const WHEEL_HINT_MS = 900;

export interface Bounds {
  x0: number;
  y0: number;
  x1: number;
  y1: number;
}

export interface ViewSize {
  w: number;
  h: number;
}

export interface Point {
  x: number;
  y: number;
}

// The subset of d3-zoom's own ZoomTransform shape this module produces and
// consumes -- {k, x, y} such that a point maps to screen space via
// `[x*k + tx, y*k + ty]`, identical to d3-zoom's own `.apply()` contract, so
// `zoomIdentity.translate(t.x, t.y).scale(t.k)` reproduces it exactly
// (translate() sets x/y first, then scale() only multiplies k, leaving x/y
// untouched -- the composition order that makes this equivalence hold).
export interface ZoomTransform {
  k: number;
  x: number;
  y: number;
}

// The subset of a real wheel/mouse/dblclick event this filter reads --
// matches both DOM events and d3-zoom's own event shape.
export interface FilterableEvent {
  type: string;
  ctrlKey?: boolean;
  metaKey?: boolean;
  button?: number;
}

// wheelZoomFilter is d3-zoom's `.filter()` (Pitfall 1, RESEARCH.md): the
// library's OWN default filter treats every wheel event as zoom-eligible
// regardless of modifiers, which is the opposite of D-20's requirement.
// A plain wheel is rejected (false) so it falls through to native page
// scroll; a wheel with ctrl/meta zooms; dblclick never zooms (re-centre
// owns it); every other gesture (drag/touch/click) passes through unless
// a non-primary mouse button is held.
export function wheelZoomFilter(event: FilterableEvent): boolean {
  if (event.type === 'dblclick') return false;
  if (event.type === 'wheel') return Boolean(event.ctrlKey || event.metaKey);
  return !event.button;
}

// fitTransform centres `bounds` inside `view` with `pad` logical units of
// margin on every side, clamping scale to [SCALE_MIN, SCALE_MAX] the same
// way the corner zoom-in/out buttons do -- so "fit" never exceeds the
// buttons' own range.
export function fitTransform(bounds: Bounds, view: ViewSize, pad = 24): ZoomTransform {
  const bw = Math.max(bounds.x1 - bounds.x0, 1e-6);
  const bh = Math.max(bounds.y1 - bounds.y0, 1e-6);
  const kRaw = Math.min(view.w / (bw + 2 * pad), view.h / (bh + 2 * pad));
  const k = Math.max(SCALE_MIN, Math.min(SCALE_MAX, kRaw));
  const cx = (bounds.x0 + bounds.x1) / 2;
  const cy = (bounds.y0 + bounds.y1) / 2;
  return { k, x: -cx * k, y: -cy * k };
}

// zoomPercent is the corner readout's number: `132` for k=1.3249, rendered
// by the caller as `{zoomPercent(k)}%`.
export function zoomPercent(k: number): number {
  return Math.round(k * 100);
}

// isOutsideView answers the auto-pan question (D-20): is `point` (in the
// graph's own untransformed coordinate space) currently outside the visible
// `view` once `t` is applied? Used to decide whether an arrow-key focus
// move should call d3-zoom's `translateTo`.
export function isOutsideView(point: Point, t: ZoomTransform, view: ViewSize): boolean {
  const sx = point.x * t.k + t.x;
  const sy = point.y * t.k + t.y;
  return Math.abs(sx) > view.w / 2 || Math.abs(sy) > view.h / 2;
}
