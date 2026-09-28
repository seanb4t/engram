import { describe, it, expect } from 'vitest';
import { wheelZoomFilter, fitTransform, zoomPercent, isOutsideView, SCALE_MIN, SCALE_MAX } from './zoom';

describe('wheelZoomFilter', () => {
  it('returns false for a plain wheel (no modifier)', () => {
    expect(wheelZoomFilter({ type: 'wheel' })).toBe(false);
  });

  it('returns false for dblclick', () => {
    expect(wheelZoomFilter({ type: 'dblclick' })).toBe(false);
  });

  it('returns true for a wheel with ctrlKey', () => {
    expect(wheelZoomFilter({ type: 'wheel', ctrlKey: true })).toBe(true);
  });

  it('returns true for a wheel with metaKey', () => {
    expect(wheelZoomFilter({ type: 'wheel', metaKey: true })).toBe(true);
  });

  it('returns true for a button-0 mousedown', () => {
    expect(wheelZoomFilter({ type: 'mousedown', button: 0 })).toBe(true);
  });

  it('returns false for a button-2 mousedown', () => {
    expect(wheelZoomFilter({ type: 'mousedown', button: 2 })).toBe(false);
  });
});

describe('fitTransform', () => {
  it('centres the bounds in the view', () => {
    const t = fitTransform({ x0: -10, y0: -10, x1: 10, y1: 10 }, { w: 440, h: 380 });
    // Bounds are already centred at (0,0) -- the fitted transform should
    // translate the centre to screen 0 too.
    expect(t.x).toBeCloseTo(0, 5);
    expect(t.y).toBeCloseTo(0, 5);
    const off = fitTransform({ x0: 40, y0: 60, x1: 60, y1: 80 }, { w: 440, h: 380 });
    // Centre (50, 70) must map to (0, 0) under the fitted transform.
    expect(50 * off.k + off.x).toBeCloseTo(0, 5);
    expect(70 * off.k + off.y).toBeCloseTo(0, 5);
  });

  it('clamps k to [SCALE_MIN, SCALE_MAX]', () => {
    // A huge bounds box would compute k far below SCALE_MIN unclamped.
    const tiny = fitTransform({ x0: -2000, y0: -2000, x1: 2000, y1: 2000 }, { w: 440, h: 380 });
    expect(tiny.k).toBe(SCALE_MIN);
    // A tiny bounds box would compute k far above SCALE_MAX unclamped.
    const huge = fitTransform({ x0: -0.001, y0: -0.001, x1: 0.001, y1: 0.001 }, { w: 440, h: 380 });
    expect(huge.k).toBe(SCALE_MAX);
  });
});

describe('zoomPercent', () => {
  it('rounds to the nearest integer percent', () => {
    expect(zoomPercent(1.3249)).toBe(132);
    expect(zoomPercent(1)).toBe(100);
    expect(zoomPercent(0.5)).toBe(50);
  });
});

describe('isOutsideView', () => {
  const view = { w: 440, h: 380 };

  it('flags a point beyond the transformed view', () => {
    expect(isOutsideView({ x: 300, y: 0 }, { k: 1, x: 0, y: 0 }, view)).toBe(true);
    expect(isOutsideView({ x: 0, y: 250 }, { k: 1, x: 0, y: 0 }, view)).toBe(true);
  });

  it('does not flag a point inside the transformed view', () => {
    expect(isOutsideView({ x: 0, y: 0 }, { k: 1, x: 0, y: 0 }, view)).toBe(false);
    expect(isOutsideView({ x: 100, y: 100 }, { k: 1, x: 0, y: 0 }, view)).toBe(false);
  });

  it('accounts for the transform, not just raw coordinates', () => {
    // At k=2, a raw point at 150 maps to 300+tx, which is outside the view;
    // the same raw point at k=1 is inside.
    expect(isOutsideView({ x: 150, y: 0 }, { k: 2, x: 0, y: 0 }, view)).toBe(true);
    expect(isOutsideView({ x: 150, y: 0 }, { k: 1, x: 0, y: 0 }, view)).toBe(false);
  });
});
