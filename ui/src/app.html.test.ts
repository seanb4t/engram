import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runInNewContext } from 'node:vm';

// app.html's inline scripts run before Svelte hydrates and cannot import
// anything (see the comment in app.html). This test extracts the script
// marked `engram:text-size` and runs it in a fresh vm context against a
// stub localStorage/document, proving the anti-flash behaviour byte-for-byte
// without a real browser.
const html = readFileSync(resolve(process.cwd(), 'src/app.html'), 'utf8');

function extractScript(marker: string): string {
  const scripts = [...html.matchAll(/<script>([\s\S]*?)<\/script>/g)].map((m) => m[1]);
  const found = scripts.find((s) => s.includes(marker));
  if (!found) throw new Error(`no inline <script> containing "${marker}" found in app.html`);
  return found;
}

function runTextSizeScript(storedValue: string | null | (() => string | null)): { uiFont: string | undefined; textSize: string | undefined } {
  const script = extractScript('engram:text-size anti-flash');
  const dataset: Record<string, string> = {};
  const properties: Record<string, string> = {};
  const context = {
    localStorage: {
      getItem: (_key: string) => (typeof storedValue === 'function' ? storedValue() : storedValue)
    },
    document: {
      documentElement: {
        style: { setProperty: (name: string, value: string) => (properties[name] = value) },
        dataset
      }
    }
  };
  runInNewContext(script, context);
  return { uiFont: properties['--ui-font'], textSize: dataset.textSize };
}

describe('app.html engram:text-size anti-flash script', () => {
  it('applies a valid stored size', () => {
    const { uiFont, textSize } = runTextSizeScript('14');
    expect(uiFont).toBe('14px');
    expect(textSize).toBe('14');
  });

  it('falls back to 15px for a non-numeric stored value', () => {
    const { uiFont, textSize } = runTextSizeScript('abc');
    expect(uiFont).toBe('15px');
    expect(textSize).toBe('15');
  });

  it('clamps an out-of-range stored value to the max', () => {
    const { uiFont, textSize } = runTextSizeScript('20');
    expect(uiFont).toBe('16px');
    expect(textSize).toBe('16');
  });

  it('falls back to 15px and throws nothing when localStorage.getItem throws', () => {
    const { uiFont, textSize } = runTextSizeScript(() => {
      throw new Error('disabled');
    });
    expect(uiFont).toBe('15px');
    expect(textSize).toBe('15');
  });
});

describe('app.html theme anti-flash script', () => {
  it('is left untouched (still reads mode-watcher-mode)', () => {
    expect(html).toContain('mode-watcher-mode');
  });
});
