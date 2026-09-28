// Console-wide display preference: text size, 12–16px, default 15, persisted per
// viewer and shared by every page (all sketches read the same key, the way every
// console route would). Sets --ui-font on <html>; surfaces size themselves off it.
(function () {
  const KEY = 'engram.console.textSize';
  const MIN = 12, MAX = 16, DEFAULT = 15;
  const clamp = (n) => Math.min(MAX, Math.max(MIN, Math.round(n)));

  function read() {
    try {
      const v = parseInt(localStorage.getItem(KEY), 10);
      return Number.isFinite(v) ? clamp(v) : DEFAULT;
    } catch (_) {
      return DEFAULT;
    }
  }

  function apply(size) {
    document.documentElement.style.setProperty('--ui-font', size + 'px');
    document.documentElement.dataset.textSize = String(size);
    window.dispatchEvent(new CustomEvent('engram:textsize', { detail: { size } }));
  }

  function set(size) {
    const s = clamp(size);
    try { localStorage.setItem(KEY, String(s)); } catch (_) { /* private window: session-only */ }
    apply(s);
    return s;
  }

  const api = {
    KEY, MIN, MAX, DEFAULT,
    get: read,
    set,
    step: (d) => set(read() + d),
    reset: () => set(DEFAULT),
  };
  window.EngramDisplay = api;

  apply(read());

  // Other open console tabs/pages follow a change made here.
  window.addEventListener('storage', (e) => { if (e.key === KEY) apply(read()); });

  // ⌘+ / ⌘- / ⌘0 (Ctrl elsewhere) step the preference instead of browser zoom.
  window.addEventListener('keydown', (e) => {
    if (!(e.metaKey || e.ctrlKey) || e.altKey) return;
    if (e.key === '=' || e.key === '+') { e.preventDefault(); api.step(1); }
    else if (e.key === '-' || e.key === '_') { e.preventDefault(); api.step(-1); }
    else if (e.key === '0') { e.preventDefault(); api.reset(); }
  });
})();
