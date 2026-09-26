// Console-wide display preference (D-13): text size, 12-16px, default 15,
// persisted per viewer and applied on every route. This is the Svelte 5
// store-module port of the sketch's `sources/themes/display.js` contract —
// keep the storage key, clamp behaviour, and event names byte-identical so
// the anti-flash script in app.html (which cannot import this module) and
// this module always agree on what a stored value means.
//
// Never throws on storage access (mirrors resume.ts's best-effort posture):
// a private-window write failure degrades to session-only persistence, not
// an error surfaced to the user.

import { toast } from 'svelte-sonner';

export const TEXT_SIZE_KEY = 'engram.console.textSize';
export const MIN_TEXT_SIZE = 12;
export const MAX_TEXT_SIZE = 16;
export const DEFAULT_TEXT_SIZE = 15;

export const display = $state<{ size: number }>({ size: DEFAULT_TEXT_SIZE });

/** Parses/rounds/clamps an arbitrary value into the valid 12-16 range. Anything unparseable (NaN, missing) falls back to the default. */
export function clampTextSize(v: unknown): number {
  const n = typeof v === 'number' ? v : parseFloat(String(v));
  if (!Number.isFinite(n)) return DEFAULT_TEXT_SIZE;
  return Math.min(MAX_TEXT_SIZE, Math.max(MIN_TEXT_SIZE, Math.round(n)));
}

/** Reads the persisted text size from the given storage, clamped. A null storage or a throwing storage both fall back to the default. */
export function readTextSize(storage: Storage | null): number {
  if (!storage) return DEFAULT_TEXT_SIZE;
  try {
    const raw = storage.getItem(TEXT_SIZE_KEY);
    if (raw === null) return DEFAULT_TEXT_SIZE;
    return clampTextSize(raw);
  } catch {
    return DEFAULT_TEXT_SIZE;
  }
}

function applyToDocument(size: number): void {
  if (typeof document === 'undefined') return;
  document.documentElement.style.setProperty('--ui-font', `${size}px`);
  document.documentElement.dataset.textSize = String(size);
}

function dispatchTextSize(size: number): void {
  if (typeof window === 'undefined') return;
  window.dispatchEvent(new CustomEvent('engram:textsize', { detail: { size } }));
}

/** Sets the text size: clamps, updates the store, applies --ui-font/data-text-size, persists best-effort, and dispatches engram:textsize. */
export function setTextSize(n: number): { size: number; atMin: boolean; atMax: boolean } {
  const size = clampTextSize(n);
  display.size = size;
  applyToDocument(size);
  try {
    if (typeof localStorage !== 'undefined') localStorage.setItem(TEXT_SIZE_KEY, String(size));
  } catch {
    // Private window / storage disabled: session-only for this tab, never an error.
  }
  dispatchTextSize(size);
  return { size, atMin: size === MIN_TEXT_SIZE, atMax: size === MAX_TEXT_SIZE };
}

export function stepTextSize(delta: 1 | -1): { size: number; atMin: boolean; atMax: boolean } {
  return setTextSize(display.size + delta);
}

export function resetTextSize(): { size: number; atMin: boolean; atMax: boolean } {
  return setTextSize(DEFAULT_TEXT_SIZE);
}

/**
 * Registers the global ⌘+/⌘-/⌘0 shortcuts and cross-tab `storage` sync on `target`.
 * Returns a cleanup function removing both listeners.
 */
export function installDisplayShortcuts(target: Window, notify: (msg: string) => void = (m) => toast(m)): () => void {
  function onKeydown(e: KeyboardEvent): void {
    if (!(e.metaKey || e.ctrlKey) || e.altKey) return;
    let result: { size: number; atMin: boolean; atMax: boolean } | null = null;
    if (e.key === '=' || e.key === '+') {
      e.preventDefault();
      result = stepTextSize(1);
    } else if (e.key === '-' || e.key === '_') {
      e.preventDefault();
      result = stepTextSize(-1);
    } else if (e.key === '0') {
      e.preventDefault();
      result = resetTextSize();
    }
    if (result) {
      const suffix = result.atMax ? ' (max)' : result.atMin ? ' (min)' : '';
      notify(`Text size ${result.size}px${suffix}`);
    }
  }

  function onStorage(e: StorageEvent): void {
    if (e.key !== TEXT_SIZE_KEY) return;
    const size = clampTextSize(e.newValue);
    display.size = size;
    applyToDocument(size);
  }

  target.addEventListener('keydown', onKeydown);
  target.addEventListener('storage', onStorage);
  return () => {
    target.removeEventListener('keydown', onKeydown);
    target.removeEventListener('storage', onStorage);
  };
}
