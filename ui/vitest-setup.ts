import '@testing-library/jest-dom/vitest';

// The node tier runs under `environment: 'node'` (no DOM), so the pure-logic
// suites need no DOM globals. localStorage still needs a stub: Node exposes a
// `localStorage` global that is `undefined` unless --localstorage-file is
// passed, and mode-watcher reads localStorage at module-evaluation time, so we
// install a minimal in-memory implementation.
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
