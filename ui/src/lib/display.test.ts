import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import {
  display,
  TEXT_SIZE_KEY,
  MIN_TEXT_SIZE,
  MAX_TEXT_SIZE,
  DEFAULT_TEXT_SIZE,
  clampTextSize,
  readTextSize,
  setTextSize,
  stepTextSize,
  resetTextSize,
  installDisplayShortcuts
} from './display.svelte';

// Node tier runs `environment: 'node'` (no DOM) -- document/localStorage are
// not Node globals, so install minimal stubs per test, mirroring
// vitest-setup.ts's localStorage stub and csrf.test.ts's document stub.
function installLocalStorageStub() {
  const store: Record<string, string> = {};
  Object.defineProperty(globalThis, 'localStorage', {
    value: {
      getItem: (k: string) => store[k] ?? null,
      setItem: (k: string, v: string) => {
        store[k] = v;
      },
      removeItem: (k: string) => {
        delete store[k];
      },
      clear: () => {
        for (const k in store) delete store[k];
      },
      get length() {
        return Object.keys(store).length;
      },
      key: (i: number) => Object.keys(store)[i] ?? null
    },
    configurable: true,
    writable: true
  });
}

function installDocumentStub() {
  const dataset: Record<string, string> = {};
  const style = { setProperty: vi.fn() };
  Object.defineProperty(globalThis, 'document', {
    value: { documentElement: { style, dataset } },
    configurable: true,
    writable: true
  });
  return { style, dataset };
}

function installWindowStub() {
  const target = new EventTarget();
  Object.defineProperty(globalThis, 'window', {
    value: target,
    configurable: true,
    writable: true
  });
  return target;
}

function keyEvent(props: Partial<{ metaKey: boolean; ctrlKey: boolean; altKey: boolean; key: string }>): Event {
  const e = new Event('keydown', { cancelable: true });
  Object.assign(e, { metaKey: false, ctrlKey: false, altKey: false, key: '', ...props });
  return e;
}

function storageEvent(key: string | null, newValue: string | null): Event {
  const e = new Event('storage');
  Object.assign(e, { key, newValue });
  return e;
}

beforeEach(() => {
  display.size = DEFAULT_TEXT_SIZE;
  installLocalStorageStub();
});

afterEach(() => {
  // installDocumentStub/installWindowStub define these as configurable globals;
  // tear them down so one test's stub can't leak into an unrelated test.
  delete (globalThis as { document?: unknown }).document;
  delete (globalThis as { window?: unknown }).window;
});

describe('clampTextSize', () => {
  it.each([
    ['14', 14],
    ['14.6', 15],
    ['99', MAX_TEXT_SIZE],
    ['3', MIN_TEXT_SIZE],
    ['abc', DEFAULT_TEXT_SIZE],
    [null, DEFAULT_TEXT_SIZE],
    [undefined, DEFAULT_TEXT_SIZE]
  ])('clampTextSize(%p) -> %p', (input, expected) => {
    expect(clampTextSize(input)).toBe(expected);
  });
});

describe('readTextSize', () => {
  it('returns the default for a null storage', () => {
    expect(readTextSize(null)).toBe(DEFAULT_TEXT_SIZE);
  });

  it('returns the clamped stored value', () => {
    const storage = { getItem: () => '13' } as unknown as Storage;
    expect(readTextSize(storage)).toBe(13);
  });

  it('returns the default when the stored value is missing or unparseable', () => {
    const storage = { getItem: () => null } as unknown as Storage;
    expect(readTextSize(storage)).toBe(DEFAULT_TEXT_SIZE);
  });

  it('returns the default when the storage getter throws', () => {
    const storage = {
      getItem: () => {
        throw new Error('disabled');
      }
    } as unknown as Storage;
    expect(readTextSize(storage)).toBe(DEFAULT_TEXT_SIZE);
  });
});

describe('setTextSize', () => {
  it('sets display.size, persists, applies --ui-font/data-text-size, and dispatches engram:textsize', () => {
    const { style, dataset } = installDocumentStub();
    const win = installWindowStub();
    const listener = vi.fn();
    win.addEventListener('engram:textsize', listener);

    const result = setTextSize(13);

    expect(result).toEqual({ size: 13, atMin: false, atMax: false });
    expect(display.size).toBe(13);
    expect(localStorage.getItem(TEXT_SIZE_KEY)).toBe('13');
    expect(style.setProperty).toHaveBeenCalledWith('--ui-font', '13px');
    expect(dataset.textSize).toBe('13');
    expect(listener).toHaveBeenCalledTimes(1);
    expect((listener.mock.calls[0][0] as CustomEvent).detail).toEqual({ size: 13 });
  });

  it('applies the size for the session without throwing when storage.setItem throws', () => {
    Object.defineProperty(globalThis, 'localStorage', {
      value: {
        getItem: () => null,
        setItem: () => {
          throw new Error('quota exceeded');
        }
      },
      configurable: true,
      writable: true
    });

    expect(() => setTextSize(14)).not.toThrow();
    expect(display.size).toBe(14);
  });
});

describe('stepTextSize / resetTextSize', () => {
  it('stepTextSize(1) at the max stays at the max and reports atMax', () => {
    setTextSize(MAX_TEXT_SIZE);
    const result = stepTextSize(1);
    expect(result).toEqual({ size: MAX_TEXT_SIZE, atMin: false, atMax: true });
  });

  it('stepTextSize(-1) at the min stays at the min and reports atMin', () => {
    setTextSize(MIN_TEXT_SIZE);
    const result = stepTextSize(-1);
    expect(result).toEqual({ size: MIN_TEXT_SIZE, atMin: true, atMax: false });
  });

  it('resetTextSize returns to the default', () => {
    setTextSize(12);
    const result = resetTextSize();
    expect(result.size).toBe(DEFAULT_TEXT_SIZE);
    expect(display.size).toBe(DEFAULT_TEXT_SIZE);
  });
});

describe('installDisplayShortcuts', () => {
  function setup() {
    const target = new EventTarget();
    const notify = vi.fn();
    const cleanup = installDisplayShortcuts(target as unknown as Window, notify);
    return { target, notify, cleanup };
  }

  it('Meta+= steps up and notifies', () => {
    const { target, notify } = setup();
    setTextSize(13);
    const e = keyEvent({ metaKey: true, key: '=' });
    const spy = vi.spyOn(e, 'preventDefault');
    target.dispatchEvent(e);
    expect(display.size).toBe(14);
    expect(spy).toHaveBeenCalled();
    expect(notify).toHaveBeenCalledWith('Text size 14px');
  });

  it('Ctrl++ steps up', () => {
    const { target, notify } = setup();
    setTextSize(13);
    target.dispatchEvent(keyEvent({ ctrlKey: true, key: '+' }));
    expect(display.size).toBe(14);
    expect(notify).toHaveBeenCalledWith('Text size 14px');
  });

  it('Meta+- steps down', () => {
    const { target, notify } = setup();
    setTextSize(14);
    target.dispatchEvent(keyEvent({ metaKey: true, key: '-' }));
    expect(display.size).toBe(13);
    expect(notify).toHaveBeenCalledWith('Text size 13px');
  });

  it('Meta+_ steps down', () => {
    const { target } = setup();
    setTextSize(14);
    target.dispatchEvent(keyEvent({ metaKey: true, key: '_' }));
    expect(display.size).toBe(13);
  });

  it('Meta+0 resets to the default', () => {
    const { target, notify } = setup();
    setTextSize(12);
    target.dispatchEvent(keyEvent({ metaKey: true, key: '0' }));
    expect(display.size).toBe(DEFAULT_TEXT_SIZE);
    expect(notify).toHaveBeenCalledWith(`Text size ${DEFAULT_TEXT_SIZE}px`);
  });

  it('appends (max) / (min) at the ends of the range', () => {
    const { target, notify } = setup();
    setTextSize(MAX_TEXT_SIZE);
    target.dispatchEvent(keyEvent({ metaKey: true, key: '=' }));
    expect(notify).toHaveBeenCalledWith(`Text size ${MAX_TEXT_SIZE}px (max)`);

    setTextSize(MIN_TEXT_SIZE);
    target.dispatchEvent(keyEvent({ metaKey: true, key: '-' }));
    expect(notify).toHaveBeenCalledWith(`Text size ${MIN_TEXT_SIZE}px (min)`);
  });

  it('ignores Alt+Meta+= and a bare =', () => {
    const { target, notify } = setup();
    target.dispatchEvent(keyEvent({ metaKey: true, altKey: true, key: '=' }));
    target.dispatchEvent(keyEvent({ key: '=' }));
    expect(display.size).toBe(DEFAULT_TEXT_SIZE);
    expect(notify).not.toHaveBeenCalled();
  });

  it('applies a storage event for the key with the new value', () => {
    const { target } = setup();
    target.dispatchEvent(storageEvent(TEXT_SIZE_KEY, '12'));
    expect(display.size).toBe(12);
  });

  it('ignores a storage event for a different key', () => {
    const { target } = setup();
    target.dispatchEvent(storageEvent('some.other.key', '12'));
    expect(display.size).toBe(DEFAULT_TEXT_SIZE);
  });

  it('cleanup removes both listeners', () => {
    const { target, notify, cleanup } = setup();
    cleanup();
    target.dispatchEvent(keyEvent({ metaKey: true, key: '=' }));
    target.dispatchEvent(storageEvent(TEXT_SIZE_KEY, '12'));
    expect(notify).not.toHaveBeenCalled();
    expect(display.size).toBe(DEFAULT_TEXT_SIZE);
  });
});
