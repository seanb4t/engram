import '@testing-library/jest-dom/vitest';

// bits-ui ScrollArea uses ResizeObserver internally; jsdom doesn't provide it.
if (typeof ResizeObserver === 'undefined') {
  globalThis.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
}

// Node 26 exposes a `localStorage` global that is undefined unless
// --localstorage-file is passed. mode-watcher reads localStorage at module
// evaluation time (before jsdom patches globals), so we provide a minimal
// in-memory stub to satisfy it in the test environment.
// bits-ui Command uses scrollIntoView on DOM nodes; jsdom doesn't implement it.
if (typeof Element !== 'undefined' && !Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = function () {};
}

if (typeof localStorage === 'undefined') {
  const store: Record<string, string> = {};
  Object.defineProperty(globalThis, 'localStorage', {
    value: {
      getItem: (k: string) => store[k] ?? null,
      setItem: (k: string, v: string) => { store[k] = v; },
      removeItem: (k: string) => { delete store[k]; },
      clear: () => { for (const k in store) delete store[k]; },
      get length() { return Object.keys(store).length; },
      key: (i: number) => Object.keys(store)[i] ?? null,
    },
    writable: true,
  });
}
